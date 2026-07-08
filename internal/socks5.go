package lumine

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"

	log "github.com/moi-si/mylog"
)

const (
	socks5RepSuccess          byte = 0x00
	socks5RepServerFailure    byte = 0x01
	socks5RepConnNotAllowed   byte = 0x02
	socks5RepCmdNotSupported  byte = 0x07
	socks5RepAtypNotSupported byte = 0x08
)

func SOCKS5Accept(addr *string, serverAddr string, done chan struct{}) {
	defer func() { done <- struct{}{} }()
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

	logger := log.New(os.Stdout, "[S00000]", log.LstdFlags, logLevel)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	if listenAddr[0] == ':' {
		listenAddr = "0.0.0.0" + listenAddr
	}
	logger.Info("SOCKS5 proxy server started at", listenAddr)

	var connID uint32
	for {
		conn, err := ln.Accept()
		if err != nil {
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
	resp := []byte{0x05, rep, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := conn.Write(resp); err != nil {
		logger.Debug("Send SOCKS5 reply:", err)
	}
}

func socks5Handler(cliConn net.Conn, id uint32) {
	logger := log.New(os.Stdout, fmt.Sprintf("[S%05x]", id), log.LstdFlags, logLevel)
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
	if header[1] != 0x01 {
		logger.Error("Expected cmd CONNECT, but got", byteToString(header[1]))
		sendReply(logger, cliConn, socks5RepCmdNotSupported)
		return
	}

	var (
		originHost, dstHost string
		policy              Policy
	)
	switch header[3] {
	case 0x01: // IPv4 address
		ipBytes, err := readN(cliConn, 4)
		if err != nil {
			logger.Error("Read IPv4 address:", err)
			return
		}
		originHost = net.IP(ipBytes).String()
		var ipPolicy *Policy
		dstHost, ipPolicy, err = ipRedirect(logger, originHost)
		if err != nil {
			logger.Error("IP redirect:", err)
			sendReply(logger, cliConn, socks5RepServerFailure)
			return
		}
		if ipPolicy == nil {
			policy = defaultPolicy
		} else {
			policy = mergePolicies(ipPolicy, &defaultPolicy)
		}
	case 0x04: // IPv6 address
		ipBytes, err := readN(cliConn, 16)
		if err != nil {
			logger.Error("Read IPv6 address:", err)
			return
		}
		originHost = net.IP(ipBytes).String()
		var ipPolicy *Policy
		dstHost, ipPolicy, err = ipRedirect(logger, originHost)
		if err != nil {
			logger.Error("IP redirect:", err)
			sendReply(logger, cliConn, socks5RepServerFailure)
			return
		}
		if ipPolicy == nil {
			policy = defaultPolicy
		} else {
			policy = mergePolicies(ipPolicy, &defaultPolicy)
		}
	case 0x03: // Domain name
		lenByte, err := readN(cliConn, 1)
		if err != nil {
			logger.Error("Read domain length:", err)
			return
		}
		domainBytes, err := readN(cliConn, int(lenByte[0]))
		if err != nil {
			logger.Error("Read domain address:", err)
		}
		originHost = string(domainBytes)
		var failed, blocked bool
		dstHost, policy, failed, blocked = genPolicy(logger, originHost)
		if failed {
			sendReply(logger, cliConn, 0x01)
			return
		}
		if blocked {
			logger.Info("Connection blocked:", originHost)
			if policy.ReplyFirst == BoolTrue {
				sendReply(logger, cliConn, socks5RepSuccess)
			} else {
				sendReply(logger, cliConn, socks5RepConnNotAllowed)
			}
			return
		}
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
	oldTarget := net.JoinHostPort(originHost, strconv.FormatUint(uint64(dstPort), 10))
	logger.Info("CONNECT", oldTarget)
	logger.Info("Policy:", policy)
	if policy.Mode == ModeBlock {
		sendReply(logger, cliConn, socks5RepConnNotAllowed)
		return
	}
	if policy.Port != 0 && policy.Port != -1 {
		dstPort = uint16(policy.Port)
	}
	target := net.JoinHostPort(dstHost, formatUint(dstPort))

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
