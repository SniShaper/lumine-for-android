//go:build !windows && !linux

package core

import (
	"net"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

var errTTLDNotSupported = E.New("`ttl-d` is not supported on current system")

type TTLProbingConfig struct{}

func setTTLProbing(TTLProbingConfig) error {
	F.Println("Warning:", errTTLDNotSupported)
	return nil
}

func getFakeTTL(log.Logger, *Policy, string, bool) (int, error) {
	return unsetInt, errTTLDNotSupported
}

func desyncSend(net.Conn, bool, []byte, int, int, int, time.Duration) error {
	return errTTLDNotSupported
}
