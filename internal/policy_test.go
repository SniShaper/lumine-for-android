package lumine

import (
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/moi-si/addrtrie"
)

func TestGenPolicyWithOptions_HostSelfResolvesAndMergesFinalIPPolicy(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	origResolver := defaultResolver
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
		defaultResolver = origResolver
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()

	hostSelf := "self"
	domainMatcher.Add("example.com", &Policy{Host: &hostSelf})
	ipMatcher.Insert("203.0.113.20/32", &Policy{Mode: ModeDirect})

	defaultResolver = stubResolver{
		resolve: func(domain string, dnsMode DNSMode) (string, bool, error) {
			if domain != "example.com" {
				t.Fatalf("unexpected resolve domain: %s", domain)
			}
			return "203.0.113.20", false, nil
		},
	}

	dstHost, p, failed, blocked, _, _ := genPolicyWithOptions(newLogger("[test]"), "example.com", true)
	if failed || blocked {
		t.Fatalf("unexpected planning result failed=%v blocked=%v", failed, blocked)
	}
	if dstHost != "203.0.113.20" {
		t.Fatalf("unexpected dstHost: %s", dstHost)
	}
	if p.Mode != ModeDirect {
		t.Fatalf("expected ip policy to win after host=self resolution, got mode=%s", p.Mode)
	}
}

func TestGenPolicyWithOptions_LiteralHostStillRunsFinalIPPolicy(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()

	literalHost := "203.0.113.30"
	domainMatcher.Add("example.com", &Policy{Host: &literalHost})
	ipMatcher.Insert("203.0.113.30/32", &Policy{Mode: ModeRaw})

	dstHost, p, failed, blocked, _, _ := genPolicyWithOptions(newLogger("[test]"), "example.com", true)
	if failed || blocked {
		t.Fatalf("unexpected planning result failed=%v blocked=%v", failed, blocked)
	}
	if dstHost != "203.0.113.30" {
		t.Fatalf("unexpected dstHost: %s", dstHost)
	}
	if p.Mode != ModeRaw {
		t.Fatalf("expected final ip policy to merge for literal host, got mode=%s", p.Mode)
	}
}

func TestGenPolicyWithOptions_DisableRedirectKeepsExplicitDomainTarget(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	origResolver := defaultResolver
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
		defaultResolver = origResolver
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()

	redirectDisabled := "^override.example"
	domainMatcher.Add("example.com", &Policy{Host: &redirectDisabled})

	calledResolve := false
	defaultResolver = stubResolver{
		resolve: func(domain string, dnsMode DNSMode) (string, bool, error) {
			calledResolve = true
			return "203.0.113.99", false, nil
		},
	}

	dstHost, _, failed, blocked, _, _ := genPolicyWithOptions(newLogger("[test]"), "example.com", true)
	if failed || blocked {
		t.Fatalf("unexpected planning result failed=%v blocked=%v", failed, blocked)
	}
	if dstHost != "override.example" {
		t.Fatalf("unexpected dstHost: %s", dstHost)
	}
	if calledResolve {
		t.Fatal("explicit ^ host should bypass resolver and ip redirect")
	}
}

func TestPlanRequest_UsesRecoveredFakeDomainBeforeRouting(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	origIPMatcher := ipMatcher
	origIPv6Matcher := ipv6Matcher
	origResolver := defaultResolver
	origV4 := defaultFakeIPv4Store
	origV6 := defaultFakeIPv6Store
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
		ipMatcher = origIPMatcher
		ipv6Matcher = origIPv6Matcher
		defaultResolver = origResolver
		defaultFakeIPv4Store = origV4
		defaultFakeIPv6Store = origV6
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()
	defaultFakeIPv4Store = mustNewFakeIPStore(fakeIPv4CIDR)
	defaultFakeIPv6Store = mustNewFakeIPStore(fakeIPv6CIDR)
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()

	hostOverride := "203.0.113.40"
	domainMatcher.Add("example.com", &Policy{Host: &hostOverride})
	ipMatcher.Insert("203.0.113.40/32", &Policy{Mode: ModeDirect})

	fakeIP, err := allocateFakeIP("example.com", dns.TypeA, defaultFakeTTL)
	if err != nil {
		t.Fatalf("allocate fake ip: %v", err)
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
	if plan.TargetHost != "203.0.113.40" {
		t.Fatalf("unexpected target host: %s", plan.TargetHost)
	}
	if plan.Policy.Mode != ModeDirect {
		t.Fatalf("expected merged ip policy after recovered domain planning, got mode=%s", plan.Policy.Mode)
	}
}

func TestGenPolicyWithOptions_GFWDomainUsesFallbackTLSRF(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
	})

	defaultPolicy = Policy{Mode: ModeRaw}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	if err := gfwDomainMatcher.Add("*example.com", struct{}{}); err != nil {
		t.Fatalf("add gfw matcher: %v", err)
	}

	dstHost, p, failed, blocked, matchedDomain, _ := genPolicyWithOptions(newLogger("[test]"), "www.example.com", false)
	if failed || blocked {
		t.Fatalf("unexpected planning result failed=%v blocked=%v", failed, blocked)
	}
	if !matchedDomain {
		t.Fatal("gfw domain should be treated as matched domain flow")
	}
	if dstHost != "www.example.com" {
		t.Fatalf("unexpected dstHost: %s", dstHost)
	}
	if p.Mode != ModeTLSRF {
		t.Fatalf("expected gfw fallback to force tls-rf, got mode=%s", p.Mode)
	}
}

func TestGenPolicyWithOptions_UnmatchedDomainFallsBackToRaw(t *testing.T) {
	origDefault := defaultPolicy
	origDomainMatcher := domainMatcher
	origGFWMatcher := gfwDomainMatcher
	origGFWBypass := gfwBypassMatcher
	t.Cleanup(func() {
		defaultPolicy = origDefault
		domainMatcher = origDomainMatcher
		gfwDomainMatcher = origGFWMatcher
		gfwBypassMatcher = origGFWBypass
	})

	defaultPolicy = Policy{Mode: ModeTLSRF}
	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()

	_, p, failed, blocked, matchedDomain, matchedIP := genPolicyWithOptions(newLogger("[test]"), "plain.example", false)
	if failed || blocked {
		t.Fatalf("unexpected planning result failed=%v blocked=%v", failed, blocked)
	}
	if matchedDomain || matchedIP {
		t.Fatalf("plain domain should remain unmatched, got matchedDomain=%v matchedIP=%v", matchedDomain, matchedIP)
	}
	if p.Mode != ModeRaw {
		t.Fatalf("expected unmatched domain to fall back to raw, got mode=%s", p.Mode)
	}
}

type stubResolver struct {
	resolve func(domain string, dnsMode DNSMode) (string, bool, error)
}

func (s stubResolver) Resolve(domain string, dnsMode DNSMode) (string, bool, error) {
	if s.resolve != nil {
		return s.resolve(domain, dnsMode)
	}
	return "", false, nil
}

func (s stubResolver) Remember(domain, ip string) {}

func (s stubResolver) LookupDomain(ip string) (string, bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", false
	}
	return lookupDomainByIP(parsed.String())
}
