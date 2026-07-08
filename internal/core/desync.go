//go:build windows || linux

package core

import (
	"net"
	"sort"
	"syscall"
	"time"

	"github.com/lzpls/enimul/internal/dial"
	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/freelru"
	"github.com/lzpls/enimul/internal/log"
	"github.com/lzpls/enimul/internal/platform"
	"github.com/lzpls/enimul/internal/singleflight"
)

var (
	calcTTL         func(int) (int, error)
	ttlCache        *freelru.ShardedLRU[string, int]
	ttlProbingGroup *singleflight.Group[string, int]
)

type TTLProbingConfig struct {
	FakeTTLRules  string `json:"fake_ttl_rules"`
	SingleFlight  bool   `json:"singleflight"`
	DisableCache  bool   `json:"disable_cache"`
	CacheCapacity uint32 `json:"cache_capacity"`
}

func setTTLProbing(c TTLProbingConfig) error {
	if err := loadTTLRules(c.FakeTTLRules); err != nil {
		return err
	}
	if c.SingleFlight {
		ttlProbingGroup = new(singleflight.Group[string, int])
	}
	if !c.DisableCache {
		if c.CacheCapacity == 0 {
			c.CacheCapacity = 1024
		}
		var err error
		ttlCache, err = freelru.NewSharded[string, int](c.CacheCapacity, hashStringXXHASH)
		if err != nil {
			return E.WithStr("init TTL cache", err)
		}
	}
	return nil
}

type ttlRule struct {
	threshold int
	typ       byte
	val       int
}

func parseTTLRules(conf string) ([]ttlRule, error) {
	b := []byte(conf)
	var rules []ttlRule
	i := 0
	for i < len(b) {
		start := i
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
		if start == i {
			return nil, E.New("invalid rule: missing left number")
		}
		a := 0
		for _, c := range b[start:i] {
			a = a*10 + int(c-'0')
		}

		if i >= len(b) {
			return nil, E.New("invalid rule: missing operator")
		}
		op := b[i]
		if op != '-' && op != '=' {
			return nil, E.New("invalid operator")
		}
		i++

		start = i
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
		if start == i {
			return nil, E.New("invalid rule: missing right number")
		}
		val := 0
		for _, c := range b[start:i] {
			val = val*10 + int(c-'0')
		}

		rules = append(rules, ttlRule{
			threshold: a,
			typ:       op,
			val:       val,
		})

		if i < len(b) && b[i] == ';' {
			i++
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].threshold > rules[j].threshold
	})
	return rules, nil
}

func loadTTLRules(conf string) error {
	if conf == "" {
		calcTTL = func(ttl int) (int, error) { return ttl - 1, nil }
		return nil
	}
	rules, err := parseTTLRules(conf)
	if err != nil {
		return E.WithStr("parse ttl rules", err)
	}
	calcTTL = func(ttl int) (int, error) {
		for _, r := range rules {
			if ttl >= r.threshold {
				if r.typ == '-' {
					return ttl - r.val, nil
				}
				// r.typ == '='
				return r.val, nil
			}
		}
		return 0, E.New("no matching ttl rule")
	}
	return nil
}

func getMinimumReachableTTL(addr string, ipv6 bool, maxTTL, attempts int, dialTimeout, cacheTTL time.Duration) (int, bool, error) {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, false, err
	}

	if ttlCache != nil {
		if ttl, ok := ttlCache.Get(ip); ok {
			return ttl, true, nil
		}
	}

	ttl := -1
	if ttlProbingGroup != nil {
		ttl, err, _ = ttlProbingGroup.Do(addr, func() (int, error) {
			return probeMinimumReachableTTL(ip, addr, ipv6, maxTTL, attempts, dialTimeout, cacheTTL)
		})
	} else {
		ttl, err = probeMinimumReachableTTL(ip, addr, ipv6, maxTTL, attempts, dialTimeout, cacheTTL)
	}
	return ttl, false, err
}

func getFakeTTL(logger log.Logger, p *Policy, addr string, ipv6 bool) (int, error) {
	if p.FakeTTL != 0 && p.FakeTTL != unsetInt {
		return p.FakeTTL, nil
	}
	ttl, cached, err := getMinimumReachableTTL(addr, ipv6, p.MaxTTL, p.Attempts, p.SingleTimeout, p.TTLCacheTTL)
	if err != nil {
		return -1, E.WithStr("get minimum reachable ttl", err)
	}
	if ttl == unsetInt {
		return -1, E.New("reachable ttl not found")
	}
	if ttl, err = calcTTL(ttl); err != nil {
		return -1, E.WithStr("calculate fake ttl", err)
	}
	if logger != nil {
		if cached {
			logger.Info("Fake TTL for ", addr, " (cached): ", ttl)
		} else {
			logger.Info("Fake TTL for ", addr, ": ", ttl)
		}
	}
	return ttl, nil
}

func ttlLevelOption(isIPv6 bool) (int, int) {
	if isIPv6 {
		return syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS
	}
	return syscall.IPPROTO_IP, syscall.IP_TTL
}

func probeMinimumReachableTTL(
	ip, addr string, isIPv6 bool,
	maxTTL, attempts int,
	dialTimeout, cacheTTL time.Duration,
) (int, error) {
	type timeoutError interface {
		Timeout() bool
	}

	level, opt := ttlLevelOption(isIPv6)
	dialer := dial.NewDialer(isIPv6)
	dialer.Timeout = dialTimeout

	low, high := 1, maxTTL
	found := -1

	for low <= high {
		mid := (low + high) / 2
		dialer.Control = func(_, _ string, c syscall.RawConn) error {
			var innerErr error
			if err := c.Control(func(fd uintptr) {
				innerErr = syscall.SetsockoptInt(platform.FD(fd), level, opt, mid)
			}); err != nil {
				return E.WithStr("raw control", err)
			}
			return innerErr
		}
		var ok bool
		for range attempts {
			conn, err := dialer.Dial("tcp", addr)
			if err == nil {
				conn.Close()
				ok = true
				break
			}
			if te, ok := err.(timeoutError); !ok || !te.Timeout() {
				return unsetInt, E.WithStr("dial with ttl "+F.Int(mid), err)
			}
		}
		if ok {
			found = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	if found != -1 && ttlCache != nil && cacheTTL != 0 && cacheTTL != unsetInt {
		ttlCache.AddWithLifetime(ip, found, cacheTTL)
	}
	return found, nil
}

func desyncSend(
	conn net.Conn, isIPv6 bool,
	record []byte, sniStart, sniLen int,
	fakeTTL int, fakeSleep time.Duration,
) error {
	rawConn, err := getRawConn(conn)
	if err != nil {
		return err
	}

	level, opt := ttlLevelOption(isIPv6)
	var defaultTTL int
	var innerErr error
	if err = rawConn.Control(func(fd uintptr) {
		defaultTTL, innerErr = syscall.GetsockoptInt(platform.FD(fd), level, opt)
	}); err != nil {
		return E.WithStr("raw control", err)
	}
	if innerErr != nil {
		return E.WithStr("get default ttl", err)
	}

	cut := findLastDotOrMidPos(record, sniStart, sniLen)
	fakeData := make([]byte, cut)
	copy(fakeData, record[:sniStart])
	const minInterval = 100 * time.Millisecond
	fakeSleep = max(minInterval, fakeSleep)

	if err = sendWithNoise(
		rawConn,
		fakeData, record[:cut],
		fakeTTL, defaultTTL,
		level, opt,
		fakeSleep,
	); err != nil {
		return E.WithStr("send data with noise", err)
	}
	if _, err = conn.Write(record[cut:]); err != nil {
		return E.WithStr("send remaining data", err)
	}
	return nil
}
