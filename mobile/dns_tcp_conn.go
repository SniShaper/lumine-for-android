package mobile

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	lumine "github.com/moi-si/lumine/internal"
	log "github.com/moi-si/mylog"
)

func newLocalDNSTCPConn(logger *log.Logger) (net.Conn, error) {
	clientConn, serverConn := net.Pipe()
	go serveLocalDNSTCP(serverConn, logger)
	return clientConn, nil
}

func serveLocalDNSTCP(conn net.Conn, logger *log.Logger) {
	defer conn.Close()

	lengthBuf := make([]byte, 2)
	for {
		if _, err := io.ReadFull(conn, lengthBuf); err != nil {
			if err != io.EOF && err != io.ErrClosedPipe {
				logger.Debug("Read hijacked TCP DNS length:", err)
			}
			return
		}

		msgLen := int(binary.BigEndian.Uint16(lengthBuf))
		if msgLen <= 0 {
			logger.Debug("Ignore empty hijacked TCP DNS message")
			return
		}

		payload := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, payload); err != nil {
			logger.Debug("Read hijacked TCP DNS payload:", err)
			return
		}

		resp, err := lumine.HandleDNSQueryPacket(payload)
		if err != nil {
			logger.Error("Handle hijacked TCP DNS query:", err)
			return
		}

		frame := make([]byte, 2+len(resp))
		binary.BigEndian.PutUint16(frame[:2], uint16(len(resp)))
		copy(frame[2:], resp)
		if _, err := conn.Write(frame); err != nil {
			logger.Debug("Write hijacked TCP DNS response:", err)
			return
		}

		logger.Info("DNS TCP hijack served:", summarizeDNSPacket(payload), "resp", summarizeDNSPacket(resp), "bytes=", fmt.Sprintf("%d", msgLen))
	}
}
