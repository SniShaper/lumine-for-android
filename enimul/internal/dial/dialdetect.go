package dial

import (
	"net"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
)

func detectByDial(network, target string, timeout time.Duration) (net.IP, string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout(network, target, timeout)
	if err != nil {
		return nil, "", E.WithStr("dial detect", err)
	}
	defer conn.Close()
	switch laddr := conn.LocalAddr().(type) {
	case *net.TCPAddr:
		return laddr.IP, laddr.Zone, nil
	case *net.UDPAddr:
		return laddr.IP, laddr.Zone, nil
	}
	return nil, "", E.New("unexpected network")
}
