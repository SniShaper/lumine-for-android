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
	"sync"
	"time"

	"github.com/elastic/go-freelru"
	"github.com/miekg/dns"
	log "github.com/moi-si/mylog"
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
	recordedDNSMu   sync.Mutex
	recordedDNSIPs  = make(map[string]recordedDNSResult)
)

type recordedDNSResult struct {
	ipv4       string
	ipv4Expire time.Time
	ipv6       string
	ipv6Expire time.Time
}

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

func rememberRecordedDNSAnswers(domain string, answers []dns.RR) {
	if domain == "" || len(answers) == 0 {
		return
	}

	now := time.Now()
	domain = strings.ToLower(domain)

	recordedDNSMu.Lock()
	defer recordedDNSMu.Unlock()

	entry := recordedDNSIPs[domain]
	for _, answer := range answers {
		switch rr := answer.(type) {
		case *dns.A:
			ttl := ttlOrDefault(rr.Hdr.Ttl)
			entry.ipv4 = rr.A.String()
			entry.ipv4Expire = now.Add(time.Duration(ttl) * time.Second)
			rememberDomainIPMapping(domain, entry.ipv4)
		case *dns.AAAA:
			ttl := ttlOrDefault(rr.Hdr.Ttl)
			entry.ipv6 = rr.AAAA.String()
			entry.ipv6Expire = now.Add(time.Duration(ttl) * time.Second)
			rememberDomainIPMapping(domain, entry.ipv6)
		}
	}

	if entry.ipv4 == "" && entry.ipv6 == "" {
		return
	}
	recordedDNSIPs[domain] = entry
}

func lookupRecordedDNSIP(domain string, dnsMode DNSMode) (string, bool) {
	if domain == "" {
		return "", false
	}

	now := time.Now()
	domain = strings.ToLower(domain)

	recordedDNSMu.Lock()
	defer recordedDNSMu.Unlock()

	entry, ok := recordedDNSIPs[domain]
	if !ok {
		return "", false
	}

	ipv4Valid := entry.ipv4 != "" && now.Before(entry.ipv4Expire)
	ipv6Valid := entry.ipv6 != "" && now.Before(entry.ipv6Expire)

	if !ipv4Valid {
		entry.ipv4 = ""
		entry.ipv4Expire = time.Time{}
	}
	if !ipv6Valid {
		entry.ipv6 = ""
		entry.ipv6Expire = time.Time{}
	}

	if entry.ipv4 == "" && entry.ipv6 == "" {
		delete(recordedDNSIPs, domain)
		return "", false
	}
	recordedDNSIPs[domain] = entry

	switch dnsMode {
	case DNSModeIPv4Only:
		if ipv4Valid {
			return entry.ipv4, true
		}
	case DNSModeIPv6Only:
		if ipv6Valid {
			return entry.ipv6, true
		}
	case DNSModePreferIPv6:
		if ipv6Valid {
			return entry.ipv6, true
		}
		if ipv4Valid {
			return entry.ipv4, true
		}
	default:
		if ipv4Valid {
			return entry.ipv4, true
		}
		if ipv6Valid {
			return entry.ipv6, true
		}
	}

	return "", false
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

func dnsLogger() *log.Logger {
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
		dnsLogger().Info("DNS HTTPS passthrough:", "domain="+domain)
		return dnsExchange(req)
	case dns.TypeA:
		return buildFakeAddressResponse(req, domain, dns.TypeA)
	case dns.TypeAAAA:
		return buildFakeAddressResponse(req, domain, dns.TypeAAAA)
	default:
		return dnsExchange(req)
	}
}

func ttlOrDefault(ttl uint32) uint32 {
	if ttl == 0 {
		return 300
	}
	return ttl
}

func buildFakeAddressResponse(req *dns.Msg, domain string, qtype uint16) (*dns.Msg, error) {
	logger := dnsLogger()
	qtypeName := dns.TypeToString[qtype]
	if qtypeName == "" {
		qtypeName = fmt.Sprintf("TYPE%d", qtype)
	}

	upstreamResp, err := dnsExchange(req)
	if err != nil {
		return nil, wrap("upstream dns exchange", err)
	}

	if upstreamResp.Rcode != dns.RcodeSuccess {
		logger.Info("DNS upstream:", domain, "type="+qtypeName, "rcode="+dns.RcodeToString[upstreamResp.Rcode])
		return upstreamResp, nil
	}

	rememberRecordedDNSAnswers(domain, upstreamResp.Answer)
	matchedRule := shouldUseFakeIP(domain)
	logger.Info(
		"DNS upstream:",
		"domain="+domain,
		"type="+qtypeName,
		"answers="+summarizeDNSAnswerSet(upstreamResp.Answer),
		"matched_rule="+boolToText(matchedRule),
	)
	if !matchedRule {
		logger.Info("DNS fake-ip skip:", "domain="+domain, "type="+qtypeName, "reason=no_rule")
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
			logger.Info(
				"DNS fake-ip allocated:",
				"domain="+domain,
				"type="+qtypeName,
				"fake="+fakeIP,
				"ttl="+formatInt(int(ttlOrDefault(header.Ttl))),
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
		logger.Info("DNS fake-ip reply:", "domain="+domain, "type="+qtypeName, "answers="+summarizeDNSAnswerSet(reply.Answer))
		return reply, nil
	}

	if len(req.Question) == 0 {
		return upstreamResp, nil
	}

	fakeIP, err = allocateFakeIP(domain, qtype, fallbackFakeTTL(upstreamResp))
	if err != nil {
		return nil, wrap("allocate fallback fake ip", err)
	}
	logger.Info(
		"DNS fake-ip allocated:",
		"domain="+domain,
		"type="+qtypeName,
		"fake="+fakeIP,
		"ttl="+formatInt(int(fallbackFakeTTL(upstreamResp))),
		"fallback=true",
	)

	fakeRR, err := buildQuestionFakeAnswerRR(req.Question[0], fakeIP, fallbackFakeTTL(upstreamResp))
	if err != nil {
		return nil, err
	}

	reply.Answer = []dns.RR{fakeRR}
	logger.Info("DNS fake-ip reply:", "domain="+domain, "type="+qtypeName, "answers="+summarizeDNSAnswerSet(reply.Answer))
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

func summarizeDNSAnswerSet(answer []dns.RR) string {
	if len(answer) == 0 {
		return "none"
	}

	limit := len(answer)
	if limit > 4 {
		limit = 4
	}

	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
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
		result += ",+" + formatInt(len(answer)-limit)
	}
	return result
}

func stringOrDefault(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}

func boolToText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
