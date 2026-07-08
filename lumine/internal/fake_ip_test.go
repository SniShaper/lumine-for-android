package lumine

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
