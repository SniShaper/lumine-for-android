package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapNAT64Addr(t *testing.T) {
	cases := []struct {
		name   string
		ip     string
		prefix string
		want   string
		ok     bool
	}{
		{"empty prefix passthrough", "151.101.195.42", "", "151.101.195.42", true},
		{"unparsable ip passthrough", "not-an-ip", "2001:67c:2960:6464::", "not-an-ip", true},
		{"native ipv6 excluded", "2001:4860:4860::8888", "2001:67c:2960:6464::", "2001:4860:4860::8888", false},
		{"bare prefix maps ipv4", "151.101.195.42", "2001:67c:2960:6464::", "2001:67c:2960:6464::9765:c32a", true},
		{"cidr prefix maps ipv4", "151.101.195.42", "2001:67c:2960:6464::/96", "2001:67c:2960:6464::9765:c32a", true},
		{"microsoft edge", "13.107.42.13", "2a01:4f9:c010:3f02:64::", "2a01:4f9:c010:3f02:64:0:d6b:2a0d", true},
		{"invalid prefix passthrough", "151.101.195.42", "not-a-prefix", "151.101.195.42", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mapNAT64Addr(tc.ip, tc.prefix)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("mapNAT64Addr(%q, %q) = (%q, %v), want (%q, %v)",
					tc.ip, tc.prefix, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestIsNAT64Enabled(t *testing.T) {
	if isNAT64Enabled(nil) {
		t.Fatal("nil policy must not enable NAT64")
	}
	if isNAT64Enabled(&Policy{}) {
		t.Fatal("empty prefix must not enable NAT64")
	}
	if isNAT64Enabled(&Policy{Nat64Prefix: "  "}) {
		t.Fatal("blank prefix must not enable NAT64")
	}
	if !isNAT64Enabled(&Policy{Nat64Prefix: "2001:67c:2960:6464::"}) {
		t.Fatal("set prefix must enable NAT64")
	}
}

func TestPlanNat64DisabledOrPassthrough(t *testing.T) {
	logger := newLogger("[test]")
	p := &Policy{}
	got, err := planNat64(logger, "151.101.195.42", p)
	if err != nil {
		t.Fatalf("planNat64 disabled returned error: %v", err)
	}
	if got != "151.101.195.42" {
		t.Fatalf("planNat64 disabled changed host to %q", got)
	}

	p = &Policy{Nat64Prefix: "2001:67c:2960:6464::"}
	got, err = planNat64(logger, "151.101.195.42", p)
	if err != nil {
		t.Fatalf("planNat64 mapping returned error: %v", err)
	}
	if got != "2001:67c:2960:6464::9765:c32a" {
		t.Fatalf("planNat64 mapped to %q", got)
	}

	if _, err = planNat64(logger, "2001:4860:4860::8888", p); err == nil {
		t.Fatal("planNat64 must reject native IPv6 under NAT64 mode")
	}
}

func TestPolicyNat64PrefixJSON(t *testing.T) {
	unmarshal := func(raw string) (*Policy, error) {
		var p Policy
		err := json.Unmarshal([]byte(raw), &p)
		return &p, err
	}

	p, err := unmarshal(`{"mode":"tls-rf","nat64_prefix":"2001:67c:2960:6464::"}`)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if p.Nat64Prefix != "2001:67c:2960:6464::" {
		t.Fatalf("nat64_prefix not parsed, got %q", p.Nat64Prefix)
	}
	if p.Mode != ModeTLSRF {
		t.Fatalf("mode not parsed alongside nat64_prefix: %v", p.Mode)
	}

	p, err = unmarshal(`{"nat64_prefix":""}`)
	if err != nil {
		t.Fatalf("unmarshal empty prefix failed: %v", err)
	}
	if p.Nat64Prefix != "" {
		t.Fatalf("empty nat64_prefix must stay disabled, got %q", p.Nat64Prefix)
	}

	p, err = unmarshal(`{"nat64_prefix":"off"}`)
	if err != nil {
		t.Fatalf("unmarshal off prefix failed: %v", err)
	}
	if p.Nat64Prefix != "" {
		t.Fatalf("'off' nat64_prefix must disable, got %q", p.Nat64Prefix)
	}

	p, err = unmarshal(`{"nat64_prefix":"  2001:67c:2960:6464::  "}`)
	if err != nil {
		t.Fatalf("unmarshal padded prefix failed: %v", err)
	}
	if p.Nat64Prefix != "2001:67c:2960:6464::" {
		t.Fatalf("nat64_prefix must be trimmed, got %q", p.Nat64Prefix)
	}
}

func TestMergePoliciesNat64(t *testing.T) {
	domain := Policy{Nat64Prefix: "2001:67c:2960:6464::"}
	base := Policy{}
	merged := mergePolicies(&domain, &base)
	if merged.Nat64Prefix != "2001:67c:2960:6464::" {
		t.Fatalf("domain nat64_prefix lost after merge: %q", merged.Nat64Prefix)
	}

	domain = Policy{}
	base = Policy{Nat64Prefix: "2a01:4f9:c010:3f02:64::"}
	merged = mergePolicies(&domain, &base)
	if merged.Nat64Prefix != "2a01:4f9:c010:3f02:64::" {
		t.Fatalf("default nat64_prefix not inherited after merge: %q", merged.Nat64Prefix)
	}

	domain = Policy{Nat64Prefix: "2001:67c:2960:6464::"}
	base = Policy{Nat64Prefix: "2a01:4f9:c010:3f02:64::"}
	merged = mergePolicies(&domain, &base)
	if merged.Nat64Prefix != "2001:67c:2960:6464::" {
		t.Fatalf("explicit rule prefix must win over default: %q", merged.Nat64Prefix)
	}
}

func TestPolicyStringNat64(t *testing.T) {
	p := Policy{Nat64Prefix: "2001:67c:2960:6464::", Mode: ModeTLSRF}
	s := p.String()
	if !strings.Contains(s, "nat64=2001:67c:2960:6464::") {
		t.Fatalf("policy string missing nat64: %s", s)
	}
}
