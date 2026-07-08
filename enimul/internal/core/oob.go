package core

import (
	"syscall"

	E "github.com/lzpls/enimul/internal/errors"
)

func getRawConn(conn any) (syscall.RawConn, error) {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return nil, E.New("not a syscall.Conn")
	}
	rawConn, err := sc.SyscallConn()
	return rawConn, E.WithStr("get raw conn", err)
}
