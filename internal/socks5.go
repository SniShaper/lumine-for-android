package lumine

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"

	log "github.com/moi-si/mylog"
)

const (
	socks5RepSuccess          byte = 0x00
	socks5RepServerFailure    byte = 0x01
	socks5RepConnNotAllowed   byte = 0x02
	socks5RepCmdNotSupported  byte = 0x07
	socks5RepAtypNotSupported byte = 0x08
)

const (
	socks5CmdConnect      byte = 0x01
	socks5CmdUDPAssociate byte = 0x03
)

func SOCKS5Accept(addr *string, serverAddr string, stop <-chan struct{}) {
	var listenAddr string
	if *addr == "" {
		listenAddr = serverAddr
	} else {
		listenAddr = *addr
	}
	if listenAddr == "" {
		fmt.Println("SOCKS5 bind address is not specified")
		return
	}
	if listenAddr == "none" {
		return
	}

	logger := newLogger("[S00000]")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ln.Close()

	go func() {
		<-stop
		_ = ln.Close()
	}()

	if listenAddr[0] == ':' {
		listenAddr = "0.0.0.0" + listenAddr
	}
	logger.Info("SOCKS5 proxy server started at", listenAddr)

	var connID uint32
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stop:
				logger.Info("SOCKS5 proxy server stopped")
				return
			default:
			}
			logger.Error("Accept:", err)
		} else {
			connID += 1
			if connID > 0xFFFFF {
				connID = 0
			}
			go socks5Handler(conn, connID)
		}
	}
}

func readN(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func sendReply(logger *log.Logger, conn net.Conn, rep byte) {
	sendReplyWithAddr(logger, conn, rep, nil)
}

func sendReplyWithAddr(logger *log.Logger, conn net.Conn, rep byte, addr net.Addr) {
	resp := []byte{0x05, rep, 0x00}
	if addr == nil {
		resp = append(resp, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
	} else {
		udpAddr, ok := addr.(*net.UDPAddr)
		if !ok {
			if tcpAddr, ok := addr.(*net.TCPAddr); ok {
				udpAddr = &net.UDPAddr{IP: tcpAddr.IP, Port: tcpAddr.Port}
			}
		}
		if udpAddr == nil {
			resp = append(resp, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
		} else if ip4 := udpAddr.IP.To4(); ip4 != nil {
			resp = append(resp, 0x01)
			resp = append(resp, ip4...)
			resp = binary.BigEndian.AppendUint16(resp, uint16(udpAddr.Port))
		} else if ip16 := udpAddr.IP.To16(); ip16 != nil {
			resp = append(resp, 0x04)
			resp = append(resp, ip16...)
			resp = binary.BigEndian.AppendUint16(resp, uint16(udpAddr.Port))
		} else {
			resp = append(resp, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
		}
	}
	if _, err := conn.Write(resp); err != nil {
		logger.Debug("Send SOCKS5 reply:", err)
	}
}

func readSocksAddr(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		ipBytes, err := readN(conn, 4)
		if err != nil {
			return "", err
		}
		return net.IP(ipBytes).String(), nil
	case 0x04:
		ipBytes, err := readN(conn, 16)
		if err != nil {
			return "", err
		}
		return net.IP(ipBytes).String(), nil
	case 0x03:
		lenByte, err := readN(conn, 1)
		if err != nil {
			return "", err
		}
		domainBytes, err := readN(conn, int(lenByte[0]))
		if err != nil {
			return "", err
		}
		return string(domainBytes), nil
	default:
		return "", fmt.Errorf("invalid address type: %d", atyp)
	}
}

func encodeSocksUDPAddr(addr net.Addr) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 0, 22)
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01)
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 0x04)
			buf = append(buf, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("host too long: %s", host)
		}
		buf = append(buf, 0x03, byte(len(host)))
		buf = append(buf, host...)
	}

	buf = binary.BigEndian.AppendUint16(buf, uint16(port))
	return buf, nil
}

func decodeSocksUDPPacket(packet []byte) (net.Addr, []byte, error) {
	if len(packet) < 4 {
		return nil, nil, fmt.Errorf("udp packet too short")
	}
	if packet[2] != 0x00 {
		return nil, nil, fmt.Errorf("fragmented udp packets are not supported")
	}

	offset := 3
	var host string
	switch packet[offset] {
	case 0x01:
		if len(packet) < offset+1+4+2 {
			return nil, nil, fmt.Errorf("udp ipv4 packet too short")
		}
		host = net.IP(packet[offset+1 : offset+5]).String()
		offset += 5
	case 0x04:
		if len(packet) < offset+1+16+2 {
			return nil, nil, fmt.Errorf("udp ipv6 packet too short")
		}
		host = net.IP(packet[offset+1 : offset+17]).String()
		offset += 17
	case 0x03:
		if len(packet) < offset+2 {
			return nil, nil, fmt.Errorf("udp domain packet too short")
		}
		hostLen := int(packet[offset+1])
		if len(packet) < offset+2+hostLen+2 {
			return nil, nil, fmt.Errorf("udp domain packet too short")
		}
		host = string(packet[offset+2 : offset+2+hostLen])
		offset += 2 + hostLen
	default:
		return nil, nil, fmt.Errorf("unsupported udp atyp: %d", packet[offset])
	}

	port := int(binary.BigEndian.Uint16(packet[offset : offset+2]))
	offset += 2

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, nil, err
	}

	return addr, packet[offset:], nil
}

func handleUDPAssociate(logger *log.Logger, cliConn net.Conn) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		logger.Error("Listen UDP:", err)
		sendReply(logger, cliConn, socks5RepServerFailure)
		return
	}
	defer pc.Close()

	sendReplyWithAddr(logger, cliConn, socks5RepSuccess, pc.LocalAddr())

	done := make(chan struct{})
	defer close(done)

	var clientAddr net.Addr
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}

			if clientAddr == nil || addr.String() == clientAddr.String() {
				clientAddr = addr
				targetAddr, payload, err := decodeSocksUDPPacket(buf[:n])
				if err != nil {
					logger.Debug("Decode UDP packet:", err)
					continue
				}
				if _, err = pc.WriteTo(payload, targetAddr); err != nil {
					logger.Debug("Forward UDP packet:", err)
				}
				continue
			}

			if clientAddr == nil {
				continue
			}

			addrBytes, err := encodeSocksUDPAddr(addr)
			if err != nil {
				logger.Debug("Encode UDP addr:", err)
				continue
			}

			resp := make([]byte, 0, 3+len(addrBytes)+n)
			resp = append(resp, 0x00, 0x00, 0x00)
			resp = append(resp, addrBytes...)
			resp = append(resp, buf[:n]...)
			if _, err = pc.WriteTo(resp, clientAddr); err != nil {
				logger.Debug("Write UDP response:", err)
			}
		}
	}()

	_, _ = io.Copy(io.Discard, cliConn)
	logger.Info("UDP associate closed")
}

func socks5Handler(cliConn net.Conn, id uint32) {
	logger := newLogger(fmt.Sprintf("[S%05x]", id))
	logger.Info("Connection from", cliConn.RemoteAddr().String())

	var (
		closeHere = true
		dstConn   net.Conn
	)
	defer func() {
		if closeHere {
			if err := cliConn.Close(); err == nil {
				logger.Debug("Closed client conn")
			} else {
				logger.Debug("Close client conn:", err)
			}
		}
	}()

	header, err := readN(cliConn, 2)
	if err != nil {
		logger.Error("Read method selection:", err)
		return
	}
	if header[0] != 0x05 {
		logger.Error("Expected socks version 5, but got", byteToString(header[0]))
		return
	}
	nMethods := int(header[1])
	methods, err := readN(cliConn, nMethods)
	if err != nil {
		logger.Error("Read methods:", err)
		return
	}
	var authMethod byte = 0xFF
	if slices.Contains(methods, 0x00) {
		authMethod = 0x00
	}
	if _, err = cliConn.Write([]byte{0x05, authMethod}); err != nil {
		logger.Error("Send auth method:", err)
		return
	}
	if authMethod == 0xFF {
		logger.Error("`no auth` method not found")
		return
	}

	header, err = readN(cliConn, 4)
	if err != nil {
		logger.Error("Read request header:", err)
		return
	}
	if header[0] != 0x05 {
		logger.Error("Expected socks version 5, but got", byteToString(header[0]))
		return
	}
	cmd := header[1]
	if cmd != socks5CmdConnect && cmd != socks5CmdUDPAssociate {
		logger.Error("Expected cmd CONNECT/UDP ASSOCIATE, but got", byteToString(cmd))
		sendReply(logger, cliConn, socks5RepCmdNotSupported)
		return
	}

	var originHost string
	originHost, err = readSocksAddr(cliConn, header[3])
	if err != nil {
		logger.Error("Read destination address:", err)
		if strings.Contains(err.Error(), "invalid address type") {
			sendReply(logger, cliConn, socks5RepAtypNotSupported)
		}
		return
	}
	if cmd == socks5CmdUDPAssociate {
		if _, err = readN(cliConn, 2); err != nil {
			logger.Error("Read UDP associate port:", err)
			return
		}
		logger.Info("UDP ASSOCIATE")
		handleUDPAssociate(logger, cliConn)
		return
	}

	switch header[3] {
	case 0x01, 0x03, 0x04:
	default:
		logger.Error("Invalid address type:", byteToString(header[3]))
		sendReply(logger, cliConn, socks5RepAtypNotSupported)
		return
	}
	portBytes, err := readN(cliConn, 2)
	if err != nil {
		logger.Error("Read port:", err)
		return
	}
	dstPort := binary.BigEndian.Uint16(portBytes)
	plan, err := PlanRequest(RequestContext{
		Source: RequestSourceSOCKS5,
		Host:   originHost,
		Port:   int(dstPort),
	}, logger)
	if err != nil {
		logger.Error("Build dial plan:", err)
		sendReply(logger, cliConn, socks5RepServerFailure)
		return
	}
	policy := plan.Policy
	if plan.Blocked {
		logger.Info("Connection blocked:", originHost)
		if header[3] == 0x03 && policy.ReplyFirst == BoolTrue {
			sendReply(logger, cliConn, socks5RepSuccess)
		} else {
			sendReply(logger, cliConn, socks5RepConnNotAllowed)
		}
		return
	}
	oldTarget := net.JoinHostPort(originHost, strconv.FormatUint(uint64(dstPort), 10))
	logger.Info("CONNECT", oldTarget)
	logger.Info("Policy:", policy)
	if policy.Mode == ModeBlock {
		sendReply(logger, cliConn, socks5RepConnNotAllowed)
		return
	}
	target := plan.TargetAddress()

	replyFirst := policy.ReplyFirst == BoolTrue
	if !replyFirst {
		dstConn, err = net.DialTimeout("tcp", target, policy.ConnectTimeout)
		if err != nil {
			logger.Error("Connection failed:", err)
			sendReply(logger, cliConn, socks5RepServerFailure)
			return
		}
	}
	sendReply(logger, cliConn, socks5RepSuccess)

	closeHere = false
	handleTunnel(policy, replyFirst, dstConn, cliConn, logger, target, originHost)
}
