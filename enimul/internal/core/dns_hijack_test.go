package core

import (
	"net"
	"net/netip"
	"testing"

	"github.com/lzpls/enimul/internal/addrtrie"
	"github.com/miekg/dns"
)

func withFakeIPFixtures(t *testing.T) {
	t.Helper()
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
}

func TestHandleDNSQueryFakeAResponsePreservesUpstreamRecords(t *testing.T) {
	withFakeIPFixtures(t)
	domainMatcher.Add("www.example.com", &Policy{})

	origExchange := dnsExchange
	t.Cleanup(func() { dnsExchange = origExchange })
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.CNAME{
				Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 120},
				Target: "edge.example.net.",
			},
			&dns.A{
				Hdr: dns.RR_Header{Name: "edge.example.net.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 45},
				A:   net.IPv4(203, 0, 113, 10),
			},
		}
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("www.example.com.", dns.TypeA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 2 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}
	if _, ok := resp.Answer[0].(*dns.CNAME); !ok {
		t.Fatalf("first answer should preserve CNAME, got %T", resp.Answer[0])
	}

	fakeA, ok := resp.Answer[1].(*dns.A)
	if !ok {
		t.Fatalf("second answer should be fake A, got %T", resp.Answer[1])
	}
	if fakeA.Hdr.Name != "edge.example.net." {
		t.Fatalf("fake A should keep upstream owner name, got %q", fakeA.Hdr.Name)
	}
	if fakeA.Hdr.Ttl != 45 {
		t.Fatalf("fake A should keep upstream ttl, got %d", fakeA.Hdr.Ttl)
	}

	addr, err := netip.ParseAddr(fakeA.A.String())
	if err != nil {
		t.Fatalf("parse fake A: %v", err)
	}
	if !netip.MustParsePrefix(fakeIPv4CIDR).Contains(addr) {
		t.Fatalf("fake A should come from fake IPv4 pool, got %s", fakeA.A.String())
	}
	if domain, ok := lookupFakeDomainByIP(fakeA.A.String()); !ok || domain != "www.example.com" {
		t.Fatalf("fake IPv4 reverse lookup mismatch, got domain=%q ok=%v", domain, ok)
	}
}

func TestHandleDNSQueryFakeAAAAResponse(t *testing.T) {
	withFakeIPFixtures(t)
	domainMatcher.Add("example.com", &Policy{})

	origExchange := dnsExchange
	t.Cleanup(func() { dnsExchange = origExchange })
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.AAAA{
				Hdr:  dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 90},
				AAAA: net.ParseIP("2001:db8::10"),
			},
		}
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeAAAA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected AAAA answer count: got %d", len(resp.Answer))
	}

	fakeAAAA, ok := resp.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("answer should be fake AAAA, got %T", resp.Answer[0])
	}

	addr, err := netip.ParseAddr(fakeAAAA.AAAA.String())
	if err != nil {
		t.Fatalf("parse fake AAAA: %v", err)
	}
	if !netip.MustParsePrefix(fakeIPv6CIDR).Contains(addr) {
		t.Fatalf("fake AAAA should come from fake IPv6 pool, got %s", fakeAAAA.AAAA.String())
	}
	if domain, ok := lookupFakeDomainByIP(fakeAAAA.AAAA.String()); !ok || domain != "example.com" {
		t.Fatalf("fake IPv6 reverse lookup mismatch, got domain=%q ok=%v", domain, ok)
	}
}

func TestShouldUseFakeIPDoesNotEnablePlainDomainsWithoutRules(t *testing.T) {
	withFakeIPFixtures(t)

	if shouldUseFakeIP("plain.example") {
		t.Fatal("plain domain should not require fake-ip before upstream response inspection")
	}
}

func TestHandleDNSQueryPassesThroughWhenNoRuleMatches(t *testing.T) {
	withFakeIPFixtures(t)

	origExchange := dnsExchange
	t.Cleanup(func() { dnsExchange = origExchange })
	realIP := net.IPv4(198, 51, 100, 20)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "plain.example.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   realIP,
			},
		}
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("plain.example.", dns.TypeA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok || !a.A.Equal(realIP) {
		t.Fatalf("expected the real upstream A record to pass through unchanged, got %+v", resp.Answer[0])
	}
}
