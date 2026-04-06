//go:build !windows && !linux

package lumine

import (
	"errors"
	"net"
	"time"

	"github.com/elastic/go-freelru"
	log "github.com/moi-si/mylog"
	"golang.org/x/sync/singleflight"
)

var errTTLDNotSupported = errors.New("`ttl-d` is not supported on current system")

var (
	ttlCache        *freelru.ShardedLRU[string, int]
	ttlCacheTTL     time.Duration
	ttlSingleflight *singleflight.Group
)

func loadTTLRules(string) error {
	return nil
}

func getFakeTTL(*log.Logger, *Policy, string, bool) (ttl int, err error) {
	return -1, errTTLDNotSupported
}

func desyncSend(net.Conn, bool, []byte, int, int, int, time.Duration) error {
	return errTTLDNotSupported
}
