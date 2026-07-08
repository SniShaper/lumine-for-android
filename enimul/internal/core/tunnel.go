package core

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/lzpls/enimul/internal/dial"
	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

func handleTunnel(
	p *Policy, dstConn, cliConn net.Conn, logger log.Logger,
	oldTarget, target, originHost, originPort string,
) {
	var (
		err       error
		closeHere = true
	)
	closeBoth := func() {
		cliConn.Close()
		if dstConn != nil {
			dstConn.Close()
		}
	}
	defer func() {
		if closeHere {
			closeBoth()
		}
	}()

	if p.Mode == ModeRaw {
		if dstConn == nil {
			dstConn, err = dial.DialTCPTimeout(target, p.ConnectTimeout)
			if err != nil {
				logger.Error("Connection to ", oldTarget, " failed: ", err)
				return
			}
		}
	} else {
		br := bufio.NewReader(cliConn)
		peekBytes, err := br.Peek(10)
		if err != nil {
			if len(peekBytes) == 0 && errors.Is(err, io.EOF) {
				logger.Error("Empty tunnel")
			} else {
				logger.Error("Read first packet: ", err)
			}
			return
		}

		if peekBytes[0] == tlsRecordTypeHandshake {
			if peekBytes[1] == tlsMajorVersion {
				payloadLen := 5 + int(binary.BigEndian.Uint16(peekBytes[3:5]))
				var ok bool
				if dstConn, ok = handleTLS(logger, payloadLen,
					p, originHost, oldTarget, target, originPort,
					br, cliConn, dstConn); !ok {
					return
				}
			}
		} else if bytesHasPrefix(peekBytes,
			"GET ", "POST ", "HEAD ", "PUT ", "DELETE ",
			"OPTIONS ", "TRACE ", "PATCH ",
		) {
			req, err := http.ReadRequest(br)
			if err == nil {
				var ok bool
				if dstConn, ok = handleHTTP(logger, req,
					p, originHost, oldTarget, target,
					cliConn, dstConn); !ok {
					return
				}
			} else {
				logger.Error("Trying parsing HTTP: ", err)
			}
		} else {
			logger.Info("Unknown protocol")
		}
		if n := br.Buffered(); n > 0 {
			buf, err := br.Peek(n)
			if err != nil {
				logger.Error("Read buffered data: ", err)
				return
			}
			if _, err := dstConn.Write(buf); err != nil {
				logger.Error("Send drained buffered data: ", err)
				return
			}
		}
	}

	logger.Info("Start forwarding")
	srcTCPConn, dstTCPConn := cliConn.(*net.TCPConn), dstConn.(*net.TCPConn)
	closeHere = false
	var done atomic.Bool
	go func() {
		if _, err := io.Copy(dstTCPConn, srcTCPConn); err != nil {
			closeBoth()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Error("Forward ", cliConn.RemoteAddr(), "->", originHost, ": ", err)
			return
		}
		logger.Debug("Forward ", cliConn.RemoteAddr(), "->", originHost, " finished")
		if err := dstTCPConn.CloseWrite(); err != nil || done.Swap(true) {
			closeBoth()
		}
	}()
	go func() {
		if _, err := io.Copy(srcTCPConn, dstTCPConn); err != nil {
			closeBoth()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Error("Forward ", originHost, "->", cliConn.RemoteAddr(), ": ", err)
			return
		}
		logger.Debug("Forward ", originHost, "->", cliConn.RemoteAddr(), " finished")
		if err := srcTCPConn.CloseWrite(); err != nil || done.Swap(true) {
			closeBoth()
		}
	}()
}

func handleHTTP(
	logger log.Logger, req *http.Request,
	p *Policy, originHost, oldTarget, target string,
	cliConn, dstConn net.Conn) (_ net.Conn, _ bool) {
	var err error
	defer req.Body.Close()

	host := req.Host
	if host == "" {
		host = req.URL.Host
		if host == "" {
			host = originHost
		}
	}
	logger.Info("host=", host, " method=", req.Method, " url=", req.URL)

	if p.HttpStatus != 0 && p.HttpStatus != unsetInt {
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
			logger.Error("Send ", p.HttpStatus, ": ", err)
		} else {
			logger.Info("Sent ", statusLine)
		}
		return
	}
	if dstConn == nil {
		dstConn, err = dial.DialTCPTimeout(target, p.ConnectTimeout)
		if err != nil {
			logger.Error("Connection to ", oldTarget, " failed: ", err)
			resp := &http.Response{
				Status:        status502,
				StatusCode:    502,
				Proto:         req.Proto,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        make(http.Header),
				ContentLength: 0,
				Close:         true,
			}
			if err = resp.Write(cliConn); err != nil {
				logger.Debug("Failed to send 502: ", err)
			}
			return
		}
	}
	if err := req.Write(dstConn); err != nil {
		logger.Error("Forward HTTP request: ", err)
		return
	}
	return dstConn, true
}

func handleTLS(logger log.Logger, recordLen int,
	p *Policy, originHost, oldTarget, target, originPort string,
	br *bufio.Reader, cliConn, dstConn net.Conn) (_ net.Conn, _ bool) {
	record := make([]byte, recordLen)
	if _, err := io.ReadFull(br, record); err != nil {
		logger.Error("Read first record: ", err)
		return
	}
	prtVer, sniStart, sniLen, hasKeyShare, hasECH, err := parseClientHello(record)
	if err != nil {
		logger.Error("Parse record: ", err)
		return
	}
	if p.Mode == ModeTLSAlert {
		sendTLSAlert(logger, cliConn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
		return
	}
	if p.TLS13Only.IsTrue() && !hasKeyShare {
		logger.Info("Connection blocked: key_share missing from ClientHello")
		sendTLSAlert(logger, cliConn, prtVer, tlsAlertProtocolVersion, tlsAlertLevelFatal)
		return
	}

	var mode Mode
	if sniStart <= 0 {
		logger.Info("SNI not found")
		mode = ModeDirect
	} else if hasECH {
		logger.Info("ECH detected ", "(SNI=", record[sniStart:sniStart+sniLen], "), ignored")
		mode = ModeDirect
	} else if sniStr := string(record[sniStart : sniStart+sniLen]); originHost != sniStr {
		logger.Info("Mismatched SNI: ", sniStr)
		switch p.SniffOverrideMode {
		case SniffOverrideRouteOnly:
			if sniPolicy, exists := domainMatcher.Find(sniStr); exists {
				if sniPolicy.Mode == ModeBlock {
					logger.Info("Connection blocked: ", sniStr)
					return
				}
				if sniPolicy.Mode == ModeTLSAlert {
					logger.Info("Connection blocked (TLS alert): ", sniStr)
					sendTLSAlert(logger, cliConn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
					return
				}
				p = mergePolicies(sniPolicy, p)
				logger.Info("New policy: ", p)
			}
		case SniffOverrideAlways, SniffOverridePolicyExists:
			newDst, sniPolicy, failed, blocked, policyNotExists := genPolicy(
				logger, sniStr, false, p.SniffOverrideMode == SniffOverridePolicyExists)
			switch {
			case failed:
				logger.Error("Failed to generate SNI policy; falling back to origin")
			case policyNotExists:
				logger.Info("SNI policy not found; falling back to origin")
			default:
				if blocked {
					logger.Info("Connection blocked: ", sniStr)
					return
				}
				if sniPolicy.Mode == ModeTLSAlert {
					logger.Info("Connection blocked (TLS alert): ", sniStr)
					sendTLSAlert(logger, cliConn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
					return
				}
				logger.Info("New policy: ", sniPolicy)
				if sniPolicy.Port != 0 && sniPolicy.Port != unsetInt {
					originPort = F.Int(sniPolicy.Port)
				}
				newTarget := net.JoinHostPort(newDst, originPort)
				newConn, err := dial.DialTCPTimeout(newTarget, sniPolicy.ConnectTimeout)
				if err == nil {
					if dstConn != nil {
						dstConn.Close()
					}
					dstConn, p, target = newConn, sniPolicy, newTarget
					logger.Info("Target has been changed to ", sniStr)
				} else {
					logger.Error("Connection to ", newTarget, " failed:", err, "; falling back to origin")
				}
			}
		}
	}

	if dstConn == nil {
		dstConn, err = dial.DialTCPTimeout(target, p.ConnectTimeout)
		if err != nil {
			logger.Error("Connection to ", oldTarget, " failed: ", err)
			return
		}
	}
	if mode == ModeUnset {
		mode = p.Mode
	}

	switch mode {
	case ModeDirect, ModeRaw:
		if _, err = dstConn.Write(record); err != nil {
			logger.Error("Send ClientHello directly: ", err)
			return
		}
		logger.Info("Sent ClientHello directly")
	case ModeTLSRF:
		err = sendRecords(dstConn, record, sniStart, sniLen,
			p.NumRecords, p.NumSegments, p.MinorVer,
			p.OOB.IsTrue(), p.OOBEx.IsTrue(),
			p.WaitForAck.IsTrue(), p.SendInterval)
		if err != nil {
			logger.Error("TLS fragment: ", err)
			return
		}
		logger.Info("Sent ClientHello in fragments")
	case ModeTTLD:
		isIPv6 := target[0] == '['
		ttl, err := getFakeTTL(logger, p, target, isIPv6)
		if err != nil {
			logger.Error("Get fake TTL: ", err)
			return
		}
		if err = desyncSend(
			dstConn, isIPv6, record,
			sniStart, sniLen, ttl, p.FakeSleep,
		); err != nil {
			logger.Error("TTL desync: ", err)
			return
		}
		logger.Info("Sent ClientHello with fake packet")
	}
	return dstConn, true
}

const (
	tlsAlertLevelFatal      byte = 2
	tlsAlertAccessDenied    byte = 70
	tlsAlertProtocolVersion byte = 49
)

func sendTLSAlert(logger log.Logger, conn net.Conn, prtVer []byte, desc byte, level byte) {
	_, err := conn.Write([]byte{0x15, prtVer[0], prtVer[1], 0x0, 0x2, level, desc})
	if err != nil {
		logger.Error("Send TLS alert: ", err)
	}
}

const (
	tlsRecordTypeHandshake      = 0x16
	tlsMajorVersion             = 0x3
	tlsRecordHeaderLen          = 5
	tlsHandshakeHeaderLen       = 4
	tlsHandshakeTypeClientHello = 0x1
	tlsExtTypeSNI               = 0x0000
	tlsExtTypeKeyShare          = 0x0033
	tlsExtTypeECH               = 0x00fe
)

func parseClientHello(data []byte) (prtVer []byte, sniStart int, sniLen int, hasKeyShare, hasECH bool, err error) {
	if data[0] != tlsRecordTypeHandshake {
		return nil, -1, 0, false, false, E.New("not a TLS handshake record")
	}

	if data[1] != tlsMajorVersion {
		return nil, -1, 0, false, false, E.New("not a standard TLS record")
	}

	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < tlsRecordHeaderLen+recordLen {
		return nil, -1, 0, false, false, E.New("record length exceeds data size")
	}
	offset := tlsRecordHeaderLen

	if recordLen < tlsHandshakeHeaderLen {
		return nil, -1, 0, false, false, E.New("handshake message too short")
	}
	if data[offset] != tlsHandshakeTypeClientHello {
		return nil, -1, 0, false, false, fmt.Errorf("not a ClientHello handshake (type=%d)", data[offset])
	}
	handshakeLen := int(uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
	if handshakeLen+tlsHandshakeHeaderLen > recordLen {
		return nil, -1, 0, false, false, E.New("handshake length exceeds record length")
	}
	offset += tlsHandshakeHeaderLen

	if handshakeLen < 2+32+1 {
		return nil, -1, 0, false, false, E.New("ClientHello too short for mandatory fields")
	}
	prtVer = data[offset : offset+2]
	offset += 2 + 32
	if offset >= len(data) {
		return prtVer, -1, 0, false, false, E.New("unexpected end after Random")
	}
	sessionIDLen := int(data[offset])
	offset++
	if offset+sessionIDLen > len(data) {
		return prtVer, -1, 0, false, false, E.New("session_id length exceeds data")
	}
	offset += sessionIDLen

	if offset+2 > len(data) {
		return prtVer, -1, 0, false, false, E.New("cannot read cipher_suites length")
	}
	csLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+csLen > len(data) {
		return prtVer, -1, 0, false, false, E.New("cipher_suites exceed data")
	}
	offset += csLen

	if offset >= len(data) {
		return prtVer, -1, 0, false, false, E.New("cannot read compression_methods length")
	}
	compMethodsLen := int(data[offset])
	offset++
	if offset+compMethodsLen > len(data) {
		return prtVer, -1, 0, false, false, E.New("compression_methods exceed data")
	}
	offset += compMethodsLen

	// Extensions
	if offset+2 > len(data) {
		return prtVer, -1, 0, false, false, nil
	}
	extTotalLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+extTotalLen > len(data) {
		return prtVer, -1, 0, false, false, E.New("extensions length exceeds data")
	}
	extensionsEnd := offset + extTotalLen

	sniStart = -1

	for offset+4 <= extensionsEnd {
		extType := binary.BigEndian.Uint16(data[offset : offset+2])
		extLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		extDataStart := offset + 4
		extDataEnd := extDataStart + extLen
		if extDataEnd > extensionsEnd {
			return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("extension length exceeds extensions block")
		}

		switch extType {
		case tlsExtTypeKeyShare:
			hasKeyShare = true
		case tlsExtTypeECH:
			hasECH = true
		case tlsExtTypeSNI:
			if sniStart != -1 {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("duplicate SNI extension")
			}
			if extLen < 2 {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("malformed SNI extension (too short for list length)")
			}
			listLen := int(binary.BigEndian.Uint16(data[extDataStart : extDataStart+2]))
			if listLen+2 != extLen {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("SNI list length field mismatch")
			}
			cursor := extDataStart + 2
			if cursor+3 > extDataEnd {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("SNI entry too short")
			}
			nameType := data[cursor]
			if nameType != 0 {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("unsupported SNI name type")
			}
			nameLen := int(binary.BigEndian.Uint16(data[cursor+1 : cursor+3]))
			nameStart := cursor + 3
			nameEnd := nameStart + nameLen
			if nameEnd > extDataEnd {
				return prtVer, sniStart, sniLen, hasKeyShare, hasECH, E.New("SNI name length exceeds extension")
			}
			sniStart = nameStart
			sniLen = nameLen
		}
		offset = extDataEnd
	}
	return prtVer, sniStart, sniLen, hasKeyShare, hasECH, nil
}

func bytesHasPrefix(b []byte, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if string(b[:len(prefix)]) == prefix {
			return true
		}
	}
	return false
}
