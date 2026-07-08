package core

import (
	"fmt"
	"net"
	"strings"

	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"

	"github.com/miekg/dns"
)

// HandleDNSQueryPacket answers a raw DNS query packet captured from the
// Android tun interface. Domains matched by an explicit rule or the
// built-in GFW list get a synthetic ("fake") A/AAAA answer instead of
// their real address, so all subsequent IP-level traffic from the app can
// be captured by the tun and routed back to the real domain via
// PlanRequest's fake-IP recovery step. Everything else passes through to
// the normal upstream resolver untouched.
func HandleDNSQueryPacket(payload []byte) ([]byte, error) {
	req := new(dns.Msg)
	if err := req.Unpack(payload); err != nil {
		return nil, E.WithStr("unpack hijacked dns request", err)
	}

	resp, err := handleDNSQuery(req)
	if err != nil {
		return nil, err
	}

	wire, err := resp.Pack()
	if err != nil {
		return nil, E.WithStr("pack hijacked dns response", err)
	}
	return wire, nil
}

func dnsLogger() log.Logger {
	return newLogger("[DNS] ")
}

func handleDNSQuery(req *dns.Msg) (*dns.Msg, error) {
	if len(req.Question) != 1 {
		return dnsExchange(req)
	}

	question := req.Question[0]
	if question.Qclass != dns.ClassINET {
		return dnsExchange(req)
	}

	domain := strings.ToLower(strings.TrimSuffix(question.Name, "."))
	switch question.Qtype {
	case dns.TypeHTTPS:
		dnsLogger().Info("DNS HTTPS passthrough: domain=", domain)
		return dnsExchange(req)
	case dns.TypeA:
		return buildFakeAddressResponse(req, domain, dns.TypeA)
	case dns.TypeAAAA:
		return buildFakeAddressResponse(req, domain, dns.TypeAAAA)
	default:
		return dnsExchange(req)
	}
}

func buildFakeAddressResponse(req *dns.Msg, domain string, qtype uint16) (*dns.Msg, error) {
	logger := dnsLogger()
	qtypeName := dns.TypeToString[qtype]
	if qtypeName == "" {
		qtypeName = fmt.Sprintf("TYPE%d", qtype)
	}

	upstreamResp, err := dnsExchange(req)
	if err != nil {
		return nil, E.WithStr("upstream dns exchange", err)
	}

	if upstreamResp.Rcode != dns.RcodeSuccess {
		logger.Info("DNS upstream: domain=", domain, " type=", qtypeName, " rcode=", dns.RcodeToString[upstreamResp.Rcode])
		return upstreamResp, nil
	}

	matchedRule := shouldUseFakeIP(domain)
	logger.Info(
		"DNS upstream: domain=", domain,
		" type=", qtypeName,
		" answers=", summarizeDNSAnswerSet(upstreamResp.Answer),
		" matched_rule=", boolToText(matchedRule),
	)
	if !matchedRule {
		logger.Info("DNS fake-ip skip: domain=", domain, " type=", qtypeName, " reason=no_rule")
		return upstreamResp, nil
	}

	replaced := false
	reply := upstreamResp.Copy()
	reply.Answer = reply.Answer[:0]

	replacedNames := make(map[string]struct{})
	fakeIP := ""
	for _, rr := range upstreamResp.Answer {
		header := rr.Header()
		if header == nil || header.Rrtype != qtype {
			reply.Answer = append(reply.Answer, rr)
			continue
		}

		if fakeIP == "" {
			fakeIP, err = allocateFakeIP(domain, qtype, header.Ttl)
			if err != nil {
				return nil, E.WithStr("allocate fake ip", err)
			}
			logger.Info(
				"DNS fake-ip allocated: domain=", domain,
				" type=", qtypeName,
				" fake=", fakeIP,
				" ttl=", F.Int(int(ttlOrDefault(header.Ttl))),
			)
		}

		replaced = true
		if _, exists := replacedNames[header.Name]; exists {
			continue
		}
		replacedNames[header.Name] = struct{}{}

		fakeRR, err := buildFakeAnswerRR(header, fakeIP)
		if err != nil {
			return nil, err
		}
		reply.Answer = append(reply.Answer, fakeRR)
	}

	if replaced {
		logger.Info("DNS fake-ip reply: domain=", domain, " type=", qtypeName, " answers=", summarizeDNSAnswerSet(reply.Answer))
		return reply, nil
	}

	if len(req.Question) == 0 {
		return upstreamResp, nil
	}

	ttl := fallbackFakeTTL(upstreamResp)
	fakeIP, err = allocateFakeIP(domain, qtype, ttl)
	if err != nil {
		return nil, E.WithStr("allocate fallback fake ip", err)
	}
	logger.Info(
		"DNS fake-ip allocated: domain=", domain,
		" type=", qtypeName,
		" fake=", fakeIP,
		" ttl=", F.Int(int(ttl)),
		" fallback=true",
	)

	fakeRR, err := buildQuestionFakeAnswerRR(req.Question[0], fakeIP, ttl)
	if err != nil {
		return nil, err
	}

	reply.Answer = []dns.RR{fakeRR}
	logger.Info("DNS fake-ip reply: domain=", domain, " type=", qtypeName, " answers=", summarizeDNSAnswerSet(reply.Answer))
	return reply, nil
}

func ttlOrDefault(ttl uint32) uint32 {
	if ttl == 0 {
		return 300
	}
	return ttl
}

func buildFakeAnswerRR(header *dns.RR_Header, fakeIP string) (dns.RR, error) {
	if header == nil {
		return nil, E.New("dns header is nil")
	}

	ttl := header.Ttl
	if ttl == 0 {
		ttl = defaultFakeTTL
	}

	switch header.Rrtype {
	case dns.TypeA:
		ip := net.ParseIP(fakeIP).To4()
		if ip == nil {
			return nil, E.New("invalid fake ipv4 address")
		}
		return &dns.A{
			Hdr: dns.RR_Header{
				Name:   header.Name,
				Rrtype: dns.TypeA,
				Class:  header.Class,
				Ttl:    ttl,
			},
			A: ip,
		}, nil
	case dns.TypeAAAA:
		ip := net.ParseIP(fakeIP)
		if ip == nil || ip.To16() == nil || ip.To4() != nil {
			return nil, E.New("invalid fake ipv6 address")
		}
		return &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   header.Name,
				Rrtype: dns.TypeAAAA,
				Class:  header.Class,
				Ttl:    ttl,
			},
			AAAA: ip,
		}, nil
	default:
		return nil, E.New("unsupported fake answer type")
	}
}

func buildQuestionFakeAnswerRR(question dns.Question, fakeIP string, ttl uint32) (dns.RR, error) {
	header := &dns.RR_Header{
		Name:   question.Name,
		Rrtype: question.Qtype,
		Class:  question.Qclass,
		Ttl:    ttl,
	}
	return buildFakeAnswerRR(header, fakeIP)
}

func fallbackFakeTTL(resp *dns.Msg) uint32 {
	if resp == nil {
		return defaultFakeTTL
	}

	minTTL := uint32(0)
	consider := func(rrs []dns.RR) {
		for _, rr := range rrs {
			header := rr.Header()
			if header == nil || header.Ttl == 0 {
				continue
			}
			if minTTL == 0 || header.Ttl < minTTL {
				minTTL = header.Ttl
			}
		}
	}

	consider(resp.Answer)
	consider(resp.Ns)
	consider(resp.Extra)
	if minTTL == 0 {
		return defaultFakeTTL
	}
	return minTTL
}

func summarizeDNSAnswerSet(answer []dns.RR) string {
	if len(answer) == 0 {
		return "none"
	}

	limit := min(len(answer), 4)

	parts := make([]string, 0, limit)
	for i := range limit {
		switch rr := answer[i].(type) {
		case *dns.A:
			parts = append(parts, "A="+rr.A.String())
		case *dns.AAAA:
			parts = append(parts, "AAAA="+rr.AAAA.String())
		case *dns.CNAME:
			parts = append(parts, "CNAME="+rr.Target)
		case *dns.HTTPS:
			parts = append(parts, "HTTPS")
		case *dns.SVCB:
			parts = append(parts, "SVCB")
		default:
			header := answer[i].Header()
			if header == nil {
				parts = append(parts, "UNKNOWN")
				continue
			}
			name := dns.TypeToString[header.Rrtype]
			if name == "" {
				name = fmt.Sprintf("TYPE%d", header.Rrtype)
			}
			parts = append(parts, name)
		}
	}

	result := strings.Join(parts, ",")
	if len(answer) > limit {
		result += ",+" + F.Int(len(answer)-limit)
	}
	return result
}

func boolToText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
