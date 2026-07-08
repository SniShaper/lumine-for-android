package core

import (
	"context"
	"net"
	"testing"

	"github.com/lzpls/enimul/internal/freelru"
)

func TestResolveBootstrapHostCachesResolvedAddress(t *testing.T) {
	origCache := dnsCache
	origLookup := bootstrapLookupIP
	t.Cleanup(func() {
		dnsCache = origCache
		bootstrapLookupIP = origLookup
	})

	cache, err := freelru.NewSharded[string, string](16, hashStringXXHASH)
	if err != nil {
		t.Fatalf("init cache: %v", err)
	}
	dnsCache = cache

	lookups := 0
	bootstrapLookupIP = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		lookups++
		if host != "cdn.example.net" {
			t.Fatalf("unexpected bootstrap host: %s", host)
		}
		return []net.IPAddr{{IP: net.IPv4(203, 0, 113, 99)}}, nil
	}

	for range 2 {
		ip, err := resolveBootstrapHost("cdn.example.net")
		if err != nil {
			t.Fatalf("resolveBootstrapHost returned error: %v", err)
		}
		if ip != "203.0.113.99" {
			t.Fatalf("unexpected bootstrap ip: %s", ip)
		}
	}

	if lookups != 1 {
		t.Fatalf("expected a single system lookup (second call should hit cache), got %d", lookups)
	}
}

func TestResolveBootstrapHostPassesThroughLiteralIP(t *testing.T) {
	ip, err := resolveBootstrapHost("203.0.113.5")
	if err != nil {
		t.Fatalf("resolveBootstrapHost returned error: %v", err)
	}
	if ip != "203.0.113.5" {
		t.Fatalf("expected literal IP to pass through unchanged, got %s", ip)
	}
}

func TestPickBootstrapIPPrefersIPv4(t *testing.T) {
	ip := pickBootstrapIP([]net.IPAddr{
		{IP: net.ParseIP("2001:db8::1")},
		{IP: net.IPv4(203, 0, 113, 7)},
	})
	if ip != "203.0.113.7" {
		t.Fatalf("expected IPv4 to be preferred, got %s", ip)
	}
}

func TestPickBootstrapIPFallsBackToFirstWhenNoIPv4(t *testing.T) {
	ip := pickBootstrapIP([]net.IPAddr{
		{IP: net.ParseIP("2001:db8::1")},
		{IP: net.ParseIP("2001:db8::2")},
	})
	if ip != "2001:db8::1" {
		t.Fatalf("expected first address as fallback, got %s", ip)
	}
}
