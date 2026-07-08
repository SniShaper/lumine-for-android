package core

import "github.com/lzpls/enimul/internal/freelru"

// ipDomainCache is the general IP->domain reverse cache: any domain
// resolution (bootstrap or otherwise) remembers its answer here, so a
// later PlanRequest for a bare IP can recover the origin domain even when
// it isn't a fake-IP allocation. Populated alongside dnsCache in setDNS.
var ipDomainCache *freelru.ShardedLRU[string, string]

func rememberDomainIPMapping(domain, ip string) {
	if domain == "" || ip == "" || ipDomainCache == nil {
		return
	}
	ipDomainCache.AddWithLifetime(ip, domain, reverseMappingTTL)
}

// lookupDomainByIP checks the fake-IP store first (fake IPs never appear
// in ipDomainCache), then falls back to the general reverse cache.
func lookupDomainByIP(ip string) (string, bool) {
	if domain, ok := lookupFakeDomainByIP(ip); ok {
		return domain, true
	}
	if ip == "" || ipDomainCache == nil {
		return "", false
	}
	return ipDomainCache.Get(ip)
}

type Resolver interface {
	Resolve(domain string, mode DNSMode) (ip string, cached bool, err error)
	Remember(domain, ip string)
	LookupDomain(ip string) (domain string, ok bool)
}

type coreResolver struct{}

var defaultResolver Resolver = coreResolver{}

func (coreResolver) Resolve(domain string, mode DNSMode) (ip string, cached bool, err error) {
	ip, cached, err = dnsResolve(domain, mode, reverseMappingTTL)
	if err == nil {
		rememberDomainIPMapping(domain, ip)
	}
	return
}

func (coreResolver) Remember(domain, ip string) {
	rememberDomainIPMapping(domain, ip)
}

func (coreResolver) LookupDomain(ip string) (string, bool) {
	return lookupDomainByIP(ip)
}
