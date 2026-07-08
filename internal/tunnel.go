package lumine

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"

	log "github.com/moi-si/mylog"
)

const (
	tlsAlertLevelFatal      byte = 2
	tlsAlertAccessDenied    byte = 70
	tlsAlertProtocolVersion byte = 49
)

func handleTunnel(
	p Policy, replyFirst bool, dstConn, cliConn net.Conn, logger *log.Logger,
	target, originHost string) {
	var (
		err       error
		once      sync.Once
		cliReader io.Reader
	)
	closeBoth := func() {
		if err := cliConn.Close(); err == nil {
			logger.Debug("Closed client conn")
		} else {
			logger.Debug("Close client conn:", err)
		}
		if dstConn != nil {
			if err := dstConn.Close(); err == nil {
				logger.Debug("Closed dest conn")
			} else {
				logger.Debug("Close dest conn:", err)
			}
		}
	}
	defer once.Do(closeBoth)

	if p.Mode == ModeRaw {
		if replyFirst {
			dstConn, err = net.DialTimeout("tcp", target, p.ConnectTimeout)
			if err != nil {
				logger.Error("Connection failed:", err)
				return
			}
		}
		cliReader = cliConn
	} else {
		br := bufio.NewReader(cliConn)
		peekBytes, err := br.Peek(10)
		if err != nil {
			if len(peekBytes) == 0 && errors.Is(err, io.EOF) {
				logger.Error("Empty tunnel")
			} else {
				logger.Error("Read first packet:", err)
			}
			return
		}

		// Require the second byte to be 0x03 to avoid handling custom TLS
		// variants like mmtls.
		if peekBytes[0] == 0x16 && peekBytes[1] == 0x03 {
			payloadLen := 5 + int(binary.BigEndian.Uint16(peekBytes[3:5]))
			var ok bool
			if dstConn, ok = handleTLS(logger, payloadLen,
				p, replyFirst, originHost, target,
				br, cliConn, dstConn); !ok {
				return
			}
		} else if bytesHasPrefix(peekBytes,
			"GET ", "POST ", "HEAD ", "PUT ", "DELETE ",
			"OPTIONS ", "TRACE ", "PATCH ",
		) {
			req, err := http.ReadRequest(br)
			if err == nil {
				var ok bool
				if dstConn, ok = handleHTTP(logger, req,
					replyFirst, originHost, target,
					cliConn, dstConn); !ok {
					return
				}
			} else {
				logger.Error("Trying parsing HTTP:", err)
			}
		} else {
			logger.Info("Unknown protocol")
		}
		// br has already buffered part of the client data (from Peek). If we
		// continued reading from cliConn directly, some of the buffered data
		// would NOT be included, leading to missing bytes. Using br ensures
		// all data, both buffered and unbuffered, is consumed.
		//
		// Additionally, bufio.Reader preserves the underlying cliConn's
		// WriteTo method, allowing io.Copy to use the optimized WriteTo
		// mechanism for performance. This means io.Copy(dstConnTCP, cliReader)
		// will be just as fast as io.Copy(cliConn, dstConn).
		cliReader = br
	}

	// Get TCPConn type for CloseWrite support.
	srcConnTCP, dstConnTCP := cliConn.(*net.TCPConn), dstConn.(*net.TCPConn)
	done := make(chan struct{})
	go func() {
		if _, err := io.Copy(dstConnTCP, cliReader); err == nil {
			if err = dstConnTCP.CloseWrite(); err == nil {
				logger.Debug("Closed dest write")
			} else {
				logger.Debug("Close dest write:", err)
				once.Do(closeBoth)
			}
		} else if !isUseOfClosedConn(err) {
			logger.Error("Forward", originHost+"->"+cliConn.RemoteAddr().String()+":", err)
			once.Do(closeBoth)
		}
		close(done)
	}()
	if _, err := io.Copy(srcConnTCP, dstConnTCP); err == nil {
		if err = srcConnTCP.CloseWrite(); err == nil {
			logger.Debug("Closed client write")
		} else {
			logger.Debug("Close client write:", err)
			once.Do(closeBoth)
		}
	} else if !isUseOfClosedConn(err) {
		logger.Error("Forward", cliConn.RemoteAddr().String()+"->"+originHost+":", err)
		once.Do(closeBoth)
	}
	<-done
}

func handleHTTP(logger *log.Logger, req *http.Request,
	replyFirst bool, originHost, target string,
	cliConn, dstConn net.Conn) (newConn net.Conn, ok bool) {
	var err error
	defer func() {
		if err := req.Body.Close(); err != nil {
			logger.Debug("Close HTTP body: ", err)
		}
	}()

	host := req.Host
	if host == "" {
		host = req.URL.Host
		if host == "" {
			host = originHost
		}
	}
	logger.Info("host="+host, "method="+req.Method, "url="+req.URL.String())

	var p Policy
	if domainPolicy, exists := domainMatcher.Find(host); exists {
		p = mergePolicies(domainPolicy, &defaultPolicy)
	} else {
		p = defaultPolicy
	}
	if p.Host != nil && *p.Host != "" {
		if (*p.Host)[0] != '^' {
			_, ipPolicy, err := ipRedirect(logger, *p.Host)
			if err != nil {
				logger.Error("IP redirect:", err)
				return
			}
			if ipPolicy != nil {
				p = mergePolicies(&p, ipPolicy, &defaultPolicy)
			}
		}
	}
	if p.HttpStatus == 0 || p.HttpStatus == -1 {
		if replyFirst {
			dstConn, err = net.DialTimeout("tcp", target, p.ConnectTimeout)
			if err != nil {
				logger.Error("Connection failed:", err)
				resp := &http.Response{
					Status:        "502 Bad Gateway",
					StatusCode:    502,
					Proto:         req.Proto,
					ProtoMajor:    1,
					ProtoMinor:    1,
					Header:        make(http.Header),
					ContentLength: 0,
					Close:         true,
				}
				if err = resp.Write(cliConn); err != nil {
					logger.Debug("Failed to send 502:", err)
				}
				return
			}
		}
		if err := req.Write(dstConn); err != nil {
			logger.Error("Forward HTTP request:", err)
			return
		}
	} else {
		statusLine := strconv.Itoa(p.HttpStatus) + " " + http.StatusText(p.HttpStatus)
		resp := &http.Response{
			Status:        statusLine,
			StatusCode:    p.HttpStatus,
			Proto:         req.Proto,
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        make(http.Header),
			ContentLength: 0,
			Close:         true,
		}
		if p.HttpStatus == 301 || p.HttpStatus == 302 {
			resp.Header.Set("Location", "https://"+host+req.URL.RequestURI())
		}
		if err = resp.Write(cliConn); err != nil {
			logger.Error("Send", p.HttpStatus, err)
		} else {
			logger.Info("Sent", statusLine)
		}
		return
	}
	return dstConn, true
}

func handleTLS(logger *log.Logger, recordLen int,
	p Policy, replyFirst bool, originHost, target string,
	br *bufio.Reader, cliConn, dstConn net.Conn) (newConn net.Conn, ok bool) {
	record := make([]byte, recordLen)
	if n, err := br.Read(record); err != nil {
		logger.Error("Read first record:", err)
		return
	} else if n < int(recordLen) {
		logger.Error(joinString("Only ", n, " of ", recordLen, " bytes read"))
		return
	}
	prtVer, sniPos, sniLen, hasKeyShare, err := parseClientHello(record)
	if err != nil {
		logger.Error("Parse record:", err)
		return
	}
	if p.Mode == ModeTLSAlert {
		sendTLSAlert(logger, cliConn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
		return
	}
	if p.TLS13Only == BoolTrue && !hasKeyShare {
		logger.Info("Connection blocked: key_share missing from ClientHello")
		sendTLSAlert(logger, cliConn, prtVer, tlsAlertProtocolVersion, tlsAlertLevelFatal)
		return
	}
	if sniPos <= 0 || sniLen <= 0 {
		logger.Info("SNI not found")
		if replyFirst {
			dstConn, err = net.DialTimeout("tcp", target, p.ConnectTimeout)
			if err != nil {
				logger.Error("Connection failed:", err)
				return
			}
		}
		if _, err = dstConn.Write(record); err != nil {
			logger.Error("ClientHello sending directly:", err)
			return
		}
		logger.Info("ClientHello sent directly")
	} else {
		sniStr := string(record[sniPos : sniPos+sniLen])
		if originHost != sniStr {
			logger.Info("Server name:", sniStr)
			var sniPolicy Policy
			if domainPolicy, exists := domainMatcher.Find(sniStr); exists {
				sniPolicy = mergePolicies(domainPolicy, &defaultPolicy)
			} else {
				sniPolicy = defaultPolicy
			}
			switch sniPolicy.Mode {
			case ModeBlock:
				logger.Info("Connection blocked")
				return
			case ModeTLSAlert:
				logger.Info("Connection blocked (TLS alert)")
				sendTLSAlert(logger, cliConn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
				return
			}
		}

		if replyFirst {
			dstConn, err = net.DialTimeout("tcp", target, p.ConnectTimeout)
			if err != nil {
				logger.Error("Connection failed:", err)
				return
			}
		}
		switch p.Mode {
		case ModeDirect:
			if _, err = dstConn.Write(record); err != nil {
				logger.Error("Send ClientHello:", err)
				return
			}
			logger.Info("ClientHello sent directly")
		case ModeTLSRF:
			err = sendRecords(dstConn, record, sniPos, sniLen,
				p.NumRecords, p.NumSegments,
				p.OOB == BoolTrue, p.OOBEx == BoolTrue,
				p.ModMinorVer == BoolTrue, p.SendInterval)
			if err != nil {
				logger.Error("TLS fragment:", err)
				return
			}
			logger.Info("ClientHello sent in fragments")
		case ModeTTLD:
			ipv6 := target[0] == '['
			ttl, err := getFakeTTL(logger, &p, target, ipv6)
			if err != nil {
				logger.Error("get fake TTL:", err)
			}
			if err = desyncSend(
				dstConn, ipv6, record,
				sniPos, sniLen, ttl, p.FakeSleep,
			); err != nil {
				logger.Error("TTL desync:", err)
				return
			}
			logger.Info("ClientHello sent with fake packet")
		}
	}
	return dstConn, true
}

func sendTLSAlert(logger *log.Logger, conn net.Conn, prtVer []byte, desc byte, level byte) {
	_, err := conn.Write([]byte{0x15, prtVer[0], prtVer[1], 0x00, 0x02, level, desc})
	if err != nil {
		logger.Error("Send TLS alert:", err)
	}
}
