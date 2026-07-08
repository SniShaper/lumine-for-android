//go:build !linux

package core

import (
	"net"
	"time"
)

func waitForAck(_ bool, _ net.Conn, delay time.Duration) error {
	time.Sleep(delay)
	return nil
}
