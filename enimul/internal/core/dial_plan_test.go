package core

import (
	"net"
	"testing"

	"github.com/lzpls/enimul/internal/addrtrie"
	"github.com/miekg/dns"
)

var testLogger = newLogger("[test] ")

func withDialPlanFixtures(t *testing.T) {
	t.Helper()
	origDomainMatcher := domainMatcher
	origHostsMatcher := hostsMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	origDefaultPolicy := defaultPolicy
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	origExchange := dnsExchange
	t.Cleanup(func() {
		domainMatcher = origDomainMatcher
		hostsMatcher = origHostsMatcher
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
		defaultPolicy = origDefaultPolicy
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
		dnsExchange = origExchange
	})

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	hostsMatcher = addrtrie.NewDomainMatcher[string]()
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	defaultPolicy = Policy{Mode: ModeDirect, DNSMode: DNSModeDefault, Host: unsetString, MapTo: unsetString, Port: unsetInt}
}

func stubDNSExchangeA(ip net.IP) {
	dnsExchange = func(req *dns.Msg) (*dns.Msg, error) {
		resp := new(dns.Msg)
		resp.SetReply(req)
		if len(req.Question) == 1 && req.Question[0].Qtype == dns.TypeA {
			resp.Answer = []dns.RR{&dns.A{
				Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   ip,
			}}
		}
		return resp, nil
	}
}

// TestPlanRequestPreserveDomainTargetWithoutHostsEntry exercises exactly the
// spot flagged as highest-risk during the port: genPolicy's
// domainNotFound=true return comes with a nil *Policy and an empty dstHost
// (it deliberately stops short of resolving), which is NOT the same shape
// as lumine's old PreserveDomainTarget behavior (which always kept the
// original domain as dstHost). PlanRequest must special-case this itself.
func TestPlanRequestPreserveDomainTargetWithoutHostsEntry(t *testing.T) {
	withDialPlanFixtures(t)

	plan, err := PlanRequest(RequestContext{
		Source:           RequestSourceHTTP,
		Host:             "connect.example.com",
		Port:             443,
		DomainTargetMode: PreserveDomainTarget,
	}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if plan.Blocked {
		t.Fatal("expected plan not to be blocked")
	}
	if plan.TargetHost != "connect.example.com" {
		t.Fatalf("expected target host to fall back to the origin domain, got %q", plan.TargetHost)
	}
	if plan.TargetAddress() != "connect.example.com:443" {
		t.Fatalf("unexpected target address: %s", plan.TargetAddress())
	}
}

func TestPlanRequestAppliesGFWFallbackForUnmatchedListedDomain(t *testing.T) {
	withDialPlanFixtures(t)
	gfwDomainMatcher.Add("*blocked.example", struct{}{})
	stubDNSExchangeA(net.IPv4(203, 0, 113, 50))

	plan, err := PlanRequest(RequestContext{Host: "blocked.example", Port: 443}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if !plan.MatchedDomain {
		t.Fatal("expected GFW-list match to count as MatchedDomain")
	}
	if plan.Policy.Mode != ModeTLSRF {
		t.Fatalf("expected gfwFallbackPolicy mode tls-rf, got %s", plan.Policy.Mode)
	}
}

func TestPlanRequestAppliesPlainFallbackForUnlistedDomain(t *testing.T) {
	withDialPlanFixtures(t)
	stubDNSExchangeA(net.IPv4(203, 0, 113, 51))

	plan, err := PlanRequest(RequestContext{Host: "plain.example", Port: 443}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if plan.MatchedDomain {
		t.Fatal("expected an unlisted domain to not count as MatchedDomain")
	}
	if plan.Policy.Mode != ModeRaw {
		t.Fatalf("expected plainFallbackPolicy mode raw, got %s", plan.Policy.Mode)
	}
}

func TestPlanRequestExplicitDomainRuleSkipsFallback(t *testing.T) {
	withDialPlanFixtures(t)
	domainMatcher.Add("rule.example", &Policy{Mode: ModeDirect, Host: unsetString, MapTo: unsetString, Port: unsetInt})
	gfwDomainMatcher.Add("*rule.example", struct{}{}) // also GFW-listed; explicit rule must win
	stubDNSExchangeA(net.IPv4(203, 0, 113, 52))

	plan, err := PlanRequest(RequestContext{Host: "rule.example", Port: 443}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if !plan.MatchedDomain {
		t.Fatal("expected explicit domain_policies match")
	}
	if plan.Policy.Mode != ModeDirect {
		t.Fatalf("expected explicit rule's mode (direct) to win over GFW fallback, got %s", plan.Policy.Mode)
	}
}

// TestPlanRequestBlockedDomainDoesNotPanic guards against a nil-pointer
// dereference: genPolicy returns a nil *Policy whenever blocked=true, and
// naively dereferencing it (e.g. for MatchedDomain/MatchedIP bookkeeping)
// would crash the mobile proxy on every single blocked connection.
func TestPlanRequestBlockedDomainDoesNotPanic(t *testing.T) {
	withDialPlanFixtures(t)
	domainMatcher.Add("blocked-rule.example", &Policy{Mode: ModeBlock, Host: unsetString, MapTo: unsetString, Port: unsetInt})

	plan, err := PlanRequest(RequestContext{Host: "blocked-rule.example", Port: 443}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if !plan.Blocked {
		t.Fatal("expected plan to be blocked")
	}
}

func TestPlanRequestDirectIPTargetMatchesIPPolicy(t *testing.T) {
	withDialPlanFixtures(t)
	ipMatcher.Insert("203.0.113.0/24", &Policy{Mode: ModeDirect, Host: unsetString, MapTo: unsetString, Port: unsetInt})

	plan, err := PlanRequest(RequestContext{Host: "203.0.113.9", Port: 443}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if !plan.MatchedIP {
		t.Fatal("expected ip_policies match for a direct IP target")
	}
	if plan.TargetHost != "203.0.113.9" {
		t.Fatalf("unexpected target host: %s", plan.TargetHost)
	}
}

func TestPlanRequestRecoversFakeIPToDomain(t *testing.T) {
	withDialPlanFixtures(t)
	domainMatcher.Add("app.example", &Policy{Mode: ModeDirect, Host: unsetString, MapTo: unsetString, Port: unsetInt})
	stubDNSExchangeA(net.IPv4(203, 0, 113, 60))

	fakeIP, err := defaultFakeIPv4Store.Allocate("app.example", 0)
	if err != nil {
		t.Fatalf("allocate fake ip: %v", err)
	}

	plan, err := PlanRequest(RequestContext{
		Source: RequestSourceMobile,
		Host:   fakeIP,
		Port:   443,
	}, testLogger)
	if err != nil {
		t.Fatalf("PlanRequest returned error: %v", err)
	}
	if plan.RecoveredDomain != "app.example" {
		t.Fatalf("expected recovered domain app.example, got %q", plan.RecoveredDomain)
	}
	if !plan.MatchedDomain {
		t.Fatal("expected the recovered domain's explicit rule to be reflected in MatchedDomain")
	}
}

func TestPlanAddressIsShorthandForPlanRequest(t *testing.T) {
	withDialPlanFixtures(t)
	stubDNSExchangeA(net.IPv4(203, 0, 113, 53))

	plan, err := PlanAddress("plain.example", testLogger)
	if err != nil {
		t.Fatalf("PlanAddress returned error: %v", err)
	}
	if plan.OriginHost != "plain.example" {
		t.Fatalf("unexpected origin host: %s", plan.OriginHost)
	}
}
