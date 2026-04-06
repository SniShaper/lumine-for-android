package lumine

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-freelru"
	"github.com/miekg/dns"
	"golang.org/x/sync/singleflight"
)

type DNSMode uint8

const (
	DNSModeUnknown DNSMode = iota
	DNSModePreferIPv4
	DNSModePreferIPv6
	DNSModeIPv4Only
	DNSModeIPv6Only
	DNSModeDefault = DNSModePreferIPv4
)

func (m DNSMode) String() string {
	switch m {
	case DNSModePreferIPv4:
		return "prefer_ipv4"
	case DNSModePreferIPv6:
		return "prefer_ipv6"
	case DNSModeIPv4Only:
		return "ipv4_only"
	case DNSModeIPv6Only:
		return "ipv6_only"
	}
	return "unknown"
}

func (m *DNSMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "prefer_ipv4":
		*m = DNSModePreferIPv4
	case "prefer_ipv6":
		*m = DNSModePreferIPv6
	case "ipv4_only":
		*m = DNSModeIPv4Only
	case "ipv6_only":
		*m = DNSModeIPv6Only
	default:
		return errors.New("invalid dns_mode: " + s)
	}
	return nil
}

var (
	dnsClient       *dns.Client
	httpCli         *http.Client
	dnsExchange     func(req *dns.Msg) (resp *dns.Msg, err error)
	dnsCache        *freelru.ShardedLRU[string, string]
	ipDomainCache   *freelru.ShardedLRU[string, string]
	dnsCacheTTL     time.Duration
	dnsSingleflight *singleflight.Group
)

func do53Exchange(req *dns.Msg) (resp *dns.Msg, err error) {
	resp, _, err = dnsClient.Exchange(req, dnsAddr)
	return resp, err
}

func dohExchange(req *dns.Msg) (resp *dns.Msg, err error) {
	wire, err := req.Pack()
	if err != nil {
		return nil, wrap("pack dns request", err)
	}
	b64 := base64.RawURLEncoding.EncodeToString(wire)
	u := dnsAddr + "?dns=" + b64
	httpReq, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, wrap("build http request", err)
	}
	httpReq.Header.Set("Accept", "application/dns-message")
	httpResp, err := httpCli.Do(httpReq)
	if err != nil {
		return nil, wrap("http request", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, errors.New("bad http status: " + httpResp.Status)
	}
	respWire, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, wrap("read http body", err)
	}
	resp = new(dns.Msg)
	if err = resp.Unpack(respWire); err != nil {
		return nil, wrap("unpack dns response", err)
	}
	return
}

func pickFirstARecord(answer []dns.RR) net.IP {
	for _, ans := range answer {
		if record, ok := ans.(*dns.A); ok {
			return record.A
		}
	}
	return nil
}

func pickFirstAAAARecord(answer []dns.RR) net.IP {
	for _, ans := range answer {
		if record, ok := ans.(*dns.AAAA); ok {
			return record.AAAA
		}
	}
	return nil
}

func doDNSResolve(domain string, dnsMode DNSMode) (string, error) {
	msg := new(dns.Msg)
	switch dnsMode {
	case DNSModePreferIPv4, DNSModeIPv4Only:
		msg.SetQuestion(domain+".", dns.TypeA)
	case DNSModePreferIPv6, DNSModeIPv6Only:
		msg.SetQuestion(domain+".", dns.TypeAAAA)
	}

	resp, err := dnsExchange(msg)
	if err != nil {
		return "", wrap("dns exchange", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		return "", errors.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
	}

	var ip net.IP
	switch dnsMode {
	case DNSModeIPv4Only:
		ip = pickFirstARecord(resp.Answer)
		if ip == nil {
			return "", errors.New("A record not found")
		}
	case DNSModeIPv6Only:
		ip = pickFirstAAAARecord(resp.Answer)
		if ip == nil {
			return "", errors.New("AAAA record not found")
		}
	case DNSModePreferIPv4:
		ip = pickFirstARecord(resp.Answer)
		if ip == nil {
			msg.SetQuestion(domain+".", dns.TypeAAAA)
			resp, err2 := dnsExchange(msg)
			if err2 != nil {
				return "", fmt.Errorf("dns exchange: %w; %w", err, err2)
			}
			if resp.Rcode != dns.RcodeSuccess {
				return "", errors.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
			}
			ip = pickFirstAAAARecord(resp.Answer)
			if ip == nil {
				return "", errors.New("record not found")
			}
		}
	case DNSModePreferIPv6:
		ip = pickFirstAAAARecord(resp.Answer)
		if ip == nil {
			msg.SetQuestion(domain+".", dns.TypeA)
			resp, err2 := dnsExchange(msg)
			if err2 != nil {
				return "", fmt.Errorf("dns exchange: %w; %w", err, err2)
			}
			if resp.Rcode != dns.RcodeSuccess {
				return "", errors.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
			}
			ip = pickFirstARecord(resp.Answer)
			if ip == nil {
				return "", errors.New("record not found")
			}
		}
	}

	ipStr := ip.String()
	if dnsCache != nil {
		dnsCache.AddWithLifetime(domain, ipStr, dnsCacheTTL)
	}
	rememberDomainIPMapping(domain, ipStr)
	return ipStr, nil
}

func dnsResolve(domain string, dnsMode DNSMode) (ip string, cached bool, err error) {
	if dnsCache != nil {
		if ip, ok := dnsCache.Get(domain); ok {
			return ip, true, nil
		}
	}

	if dnsSingleflight == nil {
		ip, err = doDNSResolve(domain, dnsMode)
	} else {
		var v any
		v, err, _ = dnsSingleflight.Do(domain, func() (any, error) {
			return doDNSResolve(domain, dnsMode)
		})
		if err == nil {
			ip = v.(string)
		}
	}

	return
}

func rememberDomainIPMapping(domain, ip string) {
	if domain == "" || ip == "" || ipDomainCache == nil {
		return
	}
	ipDomainCache.AddWithLifetime(ip, domain, dnsCacheTTL)
}

func lookupDomainByIP(ip string) (string, bool) {
	if domain, ok := lookupFakeDomainByIP(ip); ok {
		return domain, true
	}
	if ip == "" || ipDomainCache == nil {
		return "", false
	}
	return ipDomainCache.Get(ip)
}

func HandleDNSQueryPacket(payload []byte) ([]byte, error) {
	req := new(dns.Msg)
	if err := req.Unpack(payload); err != nil {
		return nil, wrap("unpack hijacked dns request", err)
	}

	resp, err := handleDNSQuery(req)
	if err != nil {
		return nil, err
	}

	wire, err := resp.Pack()
	if err != nil {
		return nil, wrap("pack hijacked dns response", err)
	}
	return wire, nil
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
	case dns.TypeA:
		if !shouldUseFakeIP(domain) {
			return dnsExchange(req)
		}
		return buildFakeAddressResponse(req, domain, dns.TypeA)
	case dns.TypeAAAA:
		if !shouldUseFakeIP(domain) {
			return dnsExchange(req)
		}
		return buildFakeAddressResponse(req, domain, dns.TypeAAAA)
	default:
		return dnsExchange(req)
	}
}

func buildFakeAddressResponse(req *dns.Msg, domain string, qtype uint16) (*dns.Msg, error) {
	upstreamResp, err := dnsExchange(req)
	if err != nil {
		return nil, wrap("upstream dns exchange", err)
	}

	if upstreamResp.Rcode != dns.RcodeSuccess {
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
				return nil, wrap("allocate fake ip", err)
			}
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
		return reply, nil
	}

	if len(req.Question) == 0 {
		return upstreamResp, nil
	}

	fakeIP, err = allocateFakeIP(domain, qtype, fallbackFakeTTL(upstreamResp))
	if err != nil {
		return nil, wrap("allocate fallback fake ip", err)
	}

	fakeRR, err := buildQuestionFakeAnswerRR(req.Question[0], fakeIP, fallbackFakeTTL(upstreamResp))
	if err != nil {
		return nil, err
	}

	reply.Answer = []dns.RR{fakeRR}
	return reply, nil
}

func buildFakeAnswerRR(header *dns.RR_Header, fakeIP string) (dns.RR, error) {
	if header == nil {
		return nil, errors.New("dns header is nil")
	}

	ttl := header.Ttl
	if ttl == 0 {
		ttl = defaultFakeTTL
	}

	switch header.Rrtype {
	case dns.TypeA:
		ip := net.ParseIP(fakeIP).To4()
		if ip == nil {
			return nil, errors.New("invalid fake ipv4 address")
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
			return nil, errors.New("invalid fake ipv6 address")
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
		return nil, errors.New("unsupported fake answer type")
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
