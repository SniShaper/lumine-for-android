package lumine

type Resolver interface {
	Resolve(domain string, dnsMode DNSMode) (ip string, cached bool, err error)
	Remember(domain, ip string)
	LookupDomain(ip string) (domain string, ok bool)
}

type coreResolver struct{}

var defaultResolver Resolver = coreResolver{}

func (coreResolver) Resolve(domain string, dnsMode DNSMode) (ip string, cached bool, err error) {
	ip, cached, err = dnsResolve(domain, dnsMode)
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
