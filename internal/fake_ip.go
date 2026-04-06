package lumine

import (
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	vpnDNSIPv4Addr    = "172.19.0.2"
	vpnTunnelIPv6Addr = "fd66:6c75:6d69::1"
	fakeIPv4CIDR      = "198.18.0.0/15"
	fakeIPv6CIDR      = "fd66:6c75:6d69:100::/56"
	defaultFakeTTL    = 60
)

type fakeIPStore struct {
	mu            sync.Mutex
	prefix        netip.Prefix
	next          netip.Addr
	domainToEntry map[string]fakeIPEntry
	ipToEntry     map[string]fakeIPEntry
	now           func() time.Time
}

type fakeIPEntry struct {
	domain    string
	ip        string
	expiresAt time.Time
}

var (
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
)

func mustNewFakeIPStore(cidr string) *fakeIPStore {
	prefix := netip.MustParsePrefix(cidr)
	start := prefix.Addr().Next()
	return &fakeIPStore{
		prefix:        prefix,
		next:          start,
		domainToEntry: make(map[string]fakeIPEntry),
		ipToEntry:     make(map[string]fakeIPEntry),
		now:           time.Now,
	}
}

func VPNDNSIPv4() string {
	return vpnDNSIPv4Addr
}

func VPNTunnelIPv6() string {
	return vpnTunnelIPv6Addr
}

func IsVPNDNSAddress(host string, port int) bool {
	return host == vpnDNSIPv4Addr && port == 53
}

func (s *fakeIPStore) Allocate(domain string, lifetime time.Duration) (string, error) {
	domain = strings.ToLower(domain)
	if domain == "" {
		return "", errors.New("domain is empty")
	}
	if lifetime <= 0 {
		lifetime = time.Duration(defaultFakeTTL) * time.Second
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.cleanupExpiredLocked(now)

	if entry, ok := s.domainToEntry[domain]; ok {
		entry.expiresAt = now.Add(lifetime)
		s.domainToEntry[domain] = entry
		s.ipToEntry[entry.ip] = entry
		return entry.ip, nil
	}

	start := s.next
	candidate := start
	for {
		if !s.prefix.Contains(candidate) {
			candidate = s.prefix.Addr().Next()
		}

		ip := candidate.String()
		if entry, exists := s.ipToEntry[ip]; exists {
			if entry.expiresAt.After(now) {
				candidate = candidate.Next()
				if candidate == start {
					return "", errors.New("fake ip pool exhausted")
				}
				continue
			}
			s.deleteEntryLocked(entry)
		}

		entry := fakeIPEntry{
			domain:    domain,
			ip:        ip,
			expiresAt: now.Add(lifetime),
		}
		s.domainToEntry[domain] = entry
		s.ipToEntry[ip] = entry
		s.next = candidate.Next()
		return ip, nil
	}
}

func (s *fakeIPStore) LookupDomain(ip string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.ipToEntry[ip]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.After(s.now()) {
		s.deleteEntryLocked(entry)
		return "", false
	}
	return entry.domain, true
}

func (s *fakeIPStore) cleanupExpiredLocked(now time.Time) {
	for _, entry := range s.domainToEntry {
		if !entry.expiresAt.After(now) {
			s.deleteEntryLocked(entry)
		}
	}
}

func (s *fakeIPStore) deleteEntryLocked(entry fakeIPEntry) {
	if current, ok := s.domainToEntry[entry.domain]; ok && current.ip == entry.ip {
		delete(s.domainToEntry, entry.domain)
	}
	if current, ok := s.ipToEntry[entry.ip]; ok && current.domain == entry.domain {
		delete(s.ipToEntry, entry.ip)
	}
}

func fakeIPLifetime(ttl uint32) time.Duration {
	if ttl == 0 {
		ttl = defaultFakeTTL
	}
	lifetime := time.Duration(ttl) * time.Second
	if dnsCacheTTL > lifetime {
		return dnsCacheTTL
	}
	return lifetime
}

func lookupFakeDomainByIP(ip string) (string, bool) {
	if domain, ok := defaultFakeIPv4Store.LookupDomain(ip); ok {
		return domain, true
	}
	return defaultFakeIPv6Store.LookupDomain(ip)
}

func isFakeIPAddress(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return defaultFakeIPv4Store.prefix.Contains(addr) || defaultFakeIPv6Store.prefix.Contains(addr)
}

func allocateFakeIP(domain string, qtype uint16, ttl uint32) (string, error) {
	lifetime := fakeIPLifetime(ttl)
	switch qtype {
	case dns.TypeA:
		return defaultFakeIPv4Store.Allocate(domain, lifetime)
	case dns.TypeAAAA:
		return defaultFakeIPv6Store.Allocate(domain, lifetime)
	default:
		return "", errors.New("unsupported fake ip qtype")
	}
}
