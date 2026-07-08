//go:build unix

package lumine

import (
	"net"

	"golang.org/x/sys/unix"
)

func sendWithOOB(conn net.Conn, data []byte, oob byte) error {
	// Tested on Android; did not work as expected.
	rawConn, err := getTCPRawConn(conn)
	if err != nil {
		return err
	}

	toSend := make([]byte, len(data)+1)
	copy(toSend, data)
	toSend[len(data)] = oob

	var innerErr error
	err = rawConn.Write(func(fd uintptr) (done bool) {
		for {
			innerErr = unix.Send(int(fd), toSend, unix.MSG_OOB)
			if innerErr == unix.EINTR {
				continue
			}
			return true
		}
	})

	if err != nil {
		return wrap("raw write (send)", err)
	}
	if innerErr != nil {
		return wrap("send (MSG_OOB)", innerErr)
	}
	return nil
}
