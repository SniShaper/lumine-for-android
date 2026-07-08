package core

import (
	"testing"
	"time"
)

func TestFakeIPStoreExpiresEntriesOnLookup(t *testing.T) {
	store := mustNewFakeIPStore(fakeIPv4CIDR)
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }

	ip, err := store.Allocate("example.com", 2*time.Second)
	if err != nil {
		t.Fatalf("allocate fake ip: %v", err)
	}

	if domain, ok := store.LookupDomain(ip); !ok || domain != "example.com" {
		t.Fatalf("lookup before expiry mismatch: domain=%q ok=%v", domain, ok)
	}

	now = now.Add(3 * time.Second)
	if domain, ok := store.LookupDomain(ip); ok || domain != "" {
		t.Fatalf("lookup after expiry should miss, got domain=%q ok=%v", domain, ok)
	}
	if _, ok := store.domainToEntry["example.com"]; ok {
		t.Fatal("expired domain entry should be removed")
	}
	if _, ok := store.ipToEntry[ip]; ok {
		t.Fatal("expired ip entry should be removed")
	}
}

func TestFakeIPStoreRefreshesExistingEntryLifetime(t *testing.T) {
	store := mustNewFakeIPStore(fakeIPv4CIDR)
	now := time.Unix(200, 0)
	store.now = func() time.Time { return now }

	ip, err := store.Allocate("example.com", 2*time.Second)
	if err != nil {
		t.Fatalf("allocate fake ip: %v", err)
	}

	now = now.Add(1 * time.Second)
	refreshedIP, err := store.Allocate("example.com", 5*time.Second)
	if err != nil {
		t.Fatalf("refresh fake ip: %v", err)
	}
	if refreshedIP != ip {
		t.Fatalf("expected same fake ip on refresh, got %s want %s", refreshedIP, ip)
	}

	now = now.Add(3 * time.Second)
	if domain, ok := store.LookupDomain(ip); !ok || domain != "example.com" {
		t.Fatalf("entry should still be alive after refresh: domain=%q ok=%v", domain, ok)
	}

	now = now.Add(3 * time.Second)
	if _, ok := store.LookupDomain(ip); ok {
		t.Fatal("entry should expire after refreshed lifetime passes")
	}
}

func TestFakeIPStoreExhaustionReturnsError(t *testing.T) {
	// A /31 leaves exactly one usable candidate address after Next(), so
	// a second distinct domain must fail to allocate.
	store := mustNewFakeIPStore("198.51.100.0/31")
	now := time.Unix(300, 0)
	store.now = func() time.Time { return now }

	if _, err := store.Allocate("a.example.com", time.Minute); err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	if _, err := store.Allocate("b.example.com", time.Minute); err == nil {
		t.Fatal("expected pool exhaustion error for second distinct domain")
	}
}

func TestIsFakeIPAddress(t *testing.T) {
	origV4, origV6 := defaultFakeIPv4Store, defaultFakeIPv6Store
	t.Cleanup(func() {
		defaultFakeIPv4Store, defaultFakeIPv6Store = origV4, origV6
	})
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)

	if !isFakeIPAddress("198.18.0.1") {
		t.Fatal("expected an address inside fakeIPv4CIDR to be recognized as fake")
	}
	if isFakeIPAddress("203.0.113.1") {
		t.Fatal("expected a normal public address to not be recognized as fake")
	}
	if isFakeIPAddress("not-an-ip") {
		t.Fatal("expected an invalid address to not be recognized as fake")
	}
}
