package lumine

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/elastic/go-freelru"
)

func TestResolveBootstrapHostCachesResolvedAddress(t *testing.T) {
	origCache := dnsCache
	origTTL := dnsCacheTTL
	origLookup := bootstrapLookupIP
	t.Cleanup(func() {
		dnsCache = origCache
		dnsCacheTTL = origTTL
		bootstrapLookupIP = origLookup
	})

	cache, err := freelru.NewSharded[string, string](16, hashStringXXHASH)
	if err != nil {
		t.Fatalf("init cache: %v", err)
	}
	dnsCache = cache
	dnsCacheTTL = time.Hour

	lookups := 0
	bootstrapLookupIP = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		lookups++
		if host != "cdn.rpnet.cc" {
			t.Fatalf("unexpected bootstrap host: %s", host)
		}
		return []net.IPAddr{{IP: net.IPv4(203, 0, 113, 99)}}, nil
	}

	for i := 0; i < 2; i++ {
		ip, err := resolveBootstrapHost("cdn.rpnet.cc")
		if err != nil {
			t.Fatalf("resolveBootstrapHost returned error: %v", err)
		}
		if ip != "203.0.113.99" {
			t.Fatalf("unexpected bootstrap ip: %s", ip)
		}
	}

	if lookups != 1 {
		t.Fatalf("expected a single system lookup, got %d", lookups)
	}
}

func TestBuildDomainMatcherKeepsDistinctPolicies(t *testing.T) {
	matcher := buildDomainMatcher(map[string]Policy{
		"example.com": {Mode: ModeRaw},
		"example.org": {Mode: ModeDirect},
	})

	rawPolicy, ok := matcher.Find("example.com")
	if !ok || rawPolicy == nil {
		t.Fatal("expected example.com to match a policy")
	}
	directPolicy, ok := matcher.Find("example.org")
	if !ok || directPolicy == nil {
		t.Fatal("expected example.org to match a policy")
	}

	if rawPolicy == directPolicy {
		t.Fatal("domain policies should not share the same pointer")
	}
	if rawPolicy.Mode != ModeRaw {
		t.Fatalf("expected example.com mode=raw, got %s", rawPolicy.Mode)
	}
	if directPolicy.Mode != ModeDirect {
		t.Fatalf("expected example.org mode=direct, got %s", directPolicy.Mode)
	}
}

func TestBuildIPMatchersKeepDistinctPolicies(t *testing.T) {
	ipv4Matcher, ipv6Matcher := buildIPMatchers(map[string]Policy{
		"203.0.113.0/24": {Mode: ModeRaw},
		"2001:db8::/32":  {Mode: ModeDirect},
	})

	ipv4Policy, ok := ipv4Matcher.Find("203.0.113.7")
	if !ok || ipv4Policy == nil {
		t.Fatal("expected IPv4 policy to match")
	}
	ipv6Policy, ok := ipv6Matcher.Find("2001:db8::7")
	if !ok || ipv6Policy == nil {
		t.Fatal("expected IPv6 policy to match")
	}

	if ipv4Policy == ipv6Policy {
		t.Fatal("ip policies should not share the same pointer")
	}
	if ipv4Policy.Mode != ModeRaw {
		t.Fatalf("expected IPv4 mode=raw, got %s", ipv4Policy.Mode)
	}
	if ipv6Policy.Mode != ModeDirect {
		t.Fatalf("expected IPv6 mode=direct, got %s", ipv6Policy.Mode)
	}
}
