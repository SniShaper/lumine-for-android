package core

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"

	F "github.com/lzpls/enimul/internal/fmt"
)

func SNIServe(cmdAddr, configAddr string) {
	listenAddr := cmdAddr
	if listenAddr == "" {
		listenAddr = configAddr
	}
	if listenAddr == "" || listenAddr == "none" {
		return
	}

	logger := newLogger("SP[00000]")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("Failed to start SNI proxy server: ", err)
		return
	}
	addr := ln.Addr().String()
	logger.Info("SNI proxy server started at ", addr)
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		panic("impossible")
	}

	var connID uint32
	for {
		conn, err := ln.Accept()
		if err == nil {
			connID += 1
			if connID > maxConnID {
				connID = 1
			}
			go handleTunnelSNI(conn, connID, port)
			continue
		}
		if ne, ok := err.(net.Error); ok && ne.Temporary() {
			logger.Warn("Accept failed: ", err)
		} else {
			logger.Error("Accept failed (fatal): ", err)
			ln.Close()
			return
		}
	}
}

func handleTunnelSNI(conn net.Conn, connID uint32, port string) {
	closeHere := true
	defer func() {
		if closeHere {
			conn.Close()
		}
	}()

	logger := newLogger(F.ConnIDToHex5("SP", connID))
	logger.Info("Connection from ", conn.RemoteAddr())

	br := bufio.NewReader(conn)
	header, err := br.Peek(5)
	if err != nil {
		if len(header) == 0 && errors.Is(err, io.EOF) {
			logger.Error("Empty tunnel")
		} else {
			logger.Error("Peek header: ", err)
		}
		return
	}

	if header[0] != tlsRecordTypeHandshake || header[1] != tlsMajorVersion {
		logger.Error("Not a standard TLS ClientHello")
		return
	}
	payloadLen := 5 + int(binary.BigEndian.Uint16(header[3:5]))

	srvConn, finalDst, ok := handleTLS(logger, payloadLen, &Policy{SniffOverrideMode: SniffOverrideAlways}, "", "", "", port, br, conn, nil, true)
	if !ok {
		return
	}
	if !drainBuffered(logger, br, srvConn) {
		return
	}

	closeHere = false
	forward(logger, conn, srvConn, finalDst)
}
