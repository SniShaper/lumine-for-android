//go:build unix

package core

import (
	"net"
	"syscall"

	E "github.com/lzpls/enimul/internal/errors"
)

func sendWithOOB(conn net.Conn, data []byte, oob byte) error {
	// Tested on Android; did not work as expected.
	rawConn, err := getRawConn(conn)
	if err != nil {
		return err
	}

	buf := make([]byte, len(data)+1)
	copy(buf, data)
	buf[len(data)] = oob

	var sendErr error
	if err = rawConn.Write(func(fd uintptr) (done bool) {
	tryagain:
		sendErr = syscall.Sendto(int(fd), buf, syscall.MSG_OOB, nil)
		if sendErr == syscall.EINTR {
			goto tryagain
		}
		return true
	}); err != nil {
		return E.WithStr("raw write", err)
	}
	return E.WithStr("sendto (MSG_OOB)", sendErr)
}
