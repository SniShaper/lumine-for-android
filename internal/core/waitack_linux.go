//go:build linux

package core

import (
	"net"
	"time"

	E "github.com/lzpls/enimul/internal/errors"

	"golang.org/x/sys/unix"
)

// Modified from https://github.com/SagerNet/sing-box/blob/83b73048ff772b919af18653b78ffeaa2d48b66e/common/tlsfragment/wait_linux.go

func waitForAck(enabled bool, conn net.Conn, delay time.Duration) error {
	if !enabled {
		time.Sleep(delay)
		return nil
	}
	rawConn, err := getRawConn(conn)
	if err != nil {
		return E.WithStr("wait for ACK", err)
	}
	var innerErr error
	rawCtrlErr := rawConn.Control(func(fd uintptr) {
		start := time.Now()
		for {
			var tcpInfo *unix.TCPInfo
			tcpInfo, innerErr = unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
			if innerErr != nil {
				return
			}
			if tcpInfo.Unacked == 0 {
				if time.Since(start) <= 20*time.Millisecond {
					time.Sleep(delay)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	if rawCtrlErr != nil {
		return E.WithStr("wait for ack: raw control", rawCtrlErr)
	}
	return E.WithStr("wait for ack: get tcp info", innerErr)
}
