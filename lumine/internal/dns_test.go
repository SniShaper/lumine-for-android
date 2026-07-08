package lumine

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/moi-si/addrtrie"
)

func TestHandleDNSQueryFakeAResponsePreservesUpstreamRecords(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	domainMatcher.Add("www.example.com", &Policy{})
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    120,
				},
				Target: "edge.example.net.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "edge.example.net.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    45,
				},
				A: net.IPv4(203, 0, 113, 10),
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "edge.example.net.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    30,
				},
				A: net.IPv4(203, 0, 113, 11),
			},
		}
		resp.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns: "ns1.example.com.",
			},
		}
		resp.Extra = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "meta.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Txt: []string{"keep"},
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
		t.Fatalf("fake A should keep first replaced ttl, got %d", fakeA.Hdr.Ttl)
	}

	addr, err := netip.ParseAddr(fakeA.A.String())
	if err != nil {
		t.Fatalf("parse fake A: %v", err)
	}
	if !netip.MustParsePrefix(fakeIPv4CIDR).Contains(addr) {
		t.Fatalf("fake A should come from fake IPv4 pool, got %s", fakeA.A.String())
	}
	if len(resp.Ns) != 1 || len(resp.Extra) != 1 {
		t.Fatalf("authority/additional records should be preserved, got Ns=%d Extra=%d", len(resp.Ns), len(resp.Extra))
	}
	if domain, ok := lookupFakeDomainByIP(fakeA.A.String()); !ok || domain != "www.example.com" {
		t.Fatalf("fake IPv4 reverse lookup mismatch, got domain=%q ok=%v", domain, ok)
	}
}

func TestHandleDNSQueryFakeAAAAResponse(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	domainMatcher.Add("example.com", &Policy{})
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    90,
				},
				AAAA: net.ParseIP("2001:db8::10"),
			},
		}
		resp.Extra = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "meta.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Txt: []string{"keep-v6"},
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
	if fakeAAAA.Hdr.Name != "example.com." {
		t.Fatalf("fake AAAA should keep upstream owner name, got %q", fakeAAAA.Hdr.Name)
	}
	if fakeAAAA.Hdr.Ttl != 90 {
		t.Fatalf("fake AAAA should keep upstream ttl, got %d", fakeAAAA.Hdr.Ttl)
	}

	addr, err := netip.ParseAddr(fakeAAAA.AAAA.String())
	if err != nil {
		t.Fatalf("parse fake AAAA: %v", err)
	}
	if !netip.MustParsePrefix(fakeIPv6CIDR).Contains(addr) {
		t.Fatalf("fake AAAA should come from fake IPv6 pool, got %s", fakeAAAA.AAAA.String())
	}
	if len(resp.Extra) != 1 {
		t.Fatalf("additional records should be preserved, got %d", len(resp.Extra))
	}
	if domain, ok := lookupFakeDomainByIP(fakeAAAA.AAAA.String()); !ok || domain != "example.com" {
		t.Fatalf("fake IPv6 reverse lookup mismatch, got domain=%q ok=%v", domain, ok)
	}
}

func TestShouldUseFakeIPDoesNotEnablePlainDomainsWithoutRules(t *testing.T) {
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	t.Cleanup(func() {
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()

	if shouldUseFakeIP("plain.example") {
		t.Fatal("plain domain should not require fake-ip before upstream response inspection")
	}
}

func TestHandleDNSQueryPassesThroughWhenDomainPolicyDoesNotRequireFakeIP(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
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
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    120,
				},
				A: net.IPv4(203, 0, 113, 10),
			},
		}
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}

	realA, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer should remain real A, got %T", resp.Answer[0])
	}
	if got := realA.A.String(); got != "203.0.113.10" {
		t.Fatalf("unexpected passthrough ip: %s", got)
	}
	if _, ok := lookupFakeDomainByIP(realA.A.String()); ok {
		t.Fatalf("real ip should not be registered as fake mapping: %s", realA.A.String())
	}
}

func TestHandleDNSQuerySynthesizesFakeAWhenRuleMatchesButAnswerIsEmpty(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	domainMatcher.Add("example.com", &Policy{})
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}

	fakeA, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer should be synthesized fake A, got %T", resp.Answer[0])
	}
	if fakeA.Hdr.Name != "example.com." {
		t.Fatalf("fake A should target original question name, got %q", fakeA.Hdr.Name)
	}
	if fakeA.Hdr.Ttl != defaultFakeTTL {
		t.Fatalf("fake A should use default ttl when upstream has none, got %d", fakeA.Hdr.Ttl)
	}
	if domain, ok := lookupFakeDomainByIP(fakeA.A.String()); !ok || domain != "example.com" {
		t.Fatalf("fake IPv4 reverse lookup mismatch, got domain=%q ok=%v", domain, ok)
	}
}

func TestHandleDNSQuerySynthesizesFakeAWhenRuleMatchesButNoAAnswerExists(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	domainMatcher.Add("example.com", &Policy{})
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    120,
				},
				Target: "edge.example.net.",
			},
		}
		resp.Extra = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "meta.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Txt: []string{"keep"},
			},
		}
		return resp, nil
	}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp, err := handleDNSQuery(req)
	if err != nil {
		t.Fatalf("handleDNSQuery returned error: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected synthesized answer count: got %d", len(resp.Answer))
	}

	fakeA, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer should be synthesized fake A, got %T", resp.Answer[0])
	}
	if fakeA.Hdr.Name != "example.com." {
		t.Fatalf("fake A should target original question name, got %q", fakeA.Hdr.Name)
	}
	if fakeA.Hdr.Ttl != 60 {
		t.Fatalf("fake A should use minimum upstream ttl, got %d", fakeA.Hdr.Ttl)
	}
	if len(resp.Extra) != 1 {
		t.Fatalf("additional records should be preserved, got %d", len(resp.Extra))
	}
}

func TestHandleDNSQueryPacket_EndToEndFakeIPPlanning(t *testing.T) {
	origExchange := dnsExchange
	origResolver := defaultResolver
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	origDNSCacheTTL := dnsCacheTTL
	t.Cleanup(func() {
		dnsExchange = origExchange
		defaultResolver = origResolver
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
		dnsCacheTTL = origDNSCacheTTL
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	domainMatcher.Add("example.com", &Policy{})
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsCacheTTL = time.Hour

	const realIP = "203.0.113.10"
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		switch req.Question[0].Qtype {
		case dns.TypeA:
			resp.Answer = []dns.RR{
				&dns.A{
					Hdr: dns.RR_Header{
						Name:   "example.com.",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					A: net.ParseIP(realIP).To4(),
				},
			}
		default:
			t.Fatalf("unexpected qtype: %d", req.Question[0].Qtype)
		}
		return resp, nil
	}
	defaultResolver = coreResolver{}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	wire, err := req.Pack()
	if err != nil {
		t.Fatalf("pack request: %v", err)
	}

	respWire, err := HandleDNSQueryPacket(wire)
	if err != nil {
		t.Fatalf("HandleDNSQueryPacket returned error: %v", err)
	}

	resp := new(dns.Msg)
	if err := resp.Unpack(respWire); err != nil {
		t.Fatalf("unpack response: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}

	fakeA, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer should be fake A, got %T", resp.Answer[0])
	}
	fakeIP := fakeA.A.String()
	if fakeIP == realIP {
		t.Fatalf("expected fake IP, got real IP %s", fakeIP)
	}

	plan, err := PlanRequest(RequestContext{
		Source: RequestSourceMobile,
		Host:   fakeIP,
		Port:   443,
	}, newLogger("[test]"))
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if plan.RecoveredDomain != "example.com" {
		t.Fatalf("unexpected recovered domain: %s", plan.RecoveredDomain)
	}
	if plan.TargetHost != realIP {
		t.Fatalf("unexpected target host: got %s want %s", plan.TargetHost, realIP)
	}
	if !isFakeIPAddress(fakeIP) {
		t.Fatalf("expected %s to be recognized as fake ip", fakeIP)
	}
}

func TestHandleDNSQueryFakeIPsGFWDomainWithoutExplicitRule(t *testing.T) {
	origExchange := dnsExchange
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		dnsExchange = origExchange
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	if err := gfwDomainMatcher.Add("*example.com", struct{}{}); err != nil {
		t.Fatalf("add gfw matcher: %v", err)
	}
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    120,
				},
				A: net.IPv4(203, 0, 113, 10),
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
	if len(resp.Answer) != 1 {
		t.Fatalf("unexpected answer count: got %d", len(resp.Answer))
	}
	if _, ok := resp.Answer[0].(*dns.A); !ok {
		t.Fatalf("answer should be fake A, got %T", resp.Answer[0])
	}
}
