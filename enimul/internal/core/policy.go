package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/lzpls/enimul/internal/addrtrie"
	"github.com/lzpls/enimul/internal/dial"
	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

var (
	defaultPolicy Policy
	domainMatcher *addrtrie.DomainMatcher[*Policy]
	ipMatcher     *addrtrie.IPv4Trie[*Policy]
	ipv6Matcher   *addrtrie.IPv6Trie[*Policy]
	hostsMatcher  *addrtrie.DomainMatcher[string]
)

const (
	unsetInt    = -1
	unsetString = "\x00"
)

type SniffOverrideMode uint8

const (
	SniffOverrideUnset SniffOverrideMode = iota
	SniffOverrideOff
	SniffOverrideAlways
	SniffOverridePolicyExists
	SniffOverrideRouteOnly
)

func (m *SniffOverrideMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "off":
		*m = SniffOverrideOff
	case "always":
		*m = SniffOverrideAlways
	case "policy_exists":
		*m = SniffOverridePolicyExists
	case "route_only":
		*m = SniffOverrideRouteOnly
	default:
		return E.New("invalid sniff_override: " + s)
	}
	return nil
}

type Mode uint8

const (
	ModeUnset Mode = iota
	ModeRaw
	ModeDirect
	ModeTLSRF
	ModeTTLD
	ModeBlock
	ModeTLSAlert
	ModeDefault = ModeTLSRF
)

const (
	ModeNameRaw      = "raw"
	ModeNameDirect   = "direct"
	ModeNameTLSRF    = "tls-rf"
	ModeNameTTLD     = "ttl-d"
	ModeNameBlock    = "block"
	ModeNameTLSAlert = "tls-alert"
)

func (m *Mode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case ModeNameRaw:
		*m = ModeRaw
	case ModeNameDirect:
		*m = ModeDirect
	case ModeNameTLSRF:
		*m = ModeTLSRF
	case ModeNameTTLD:
		*m = ModeTTLD
	case ModeNameBlock:
		*m = ModeBlock
	case ModeNameTLSAlert:
		*m = ModeTLSAlert
	default:
		return E.New("invalid mode: " + s)
	}
	return nil
}

func (m Mode) String() string {
	switch m {
	case ModeRaw:
		return ModeNameRaw
	case ModeDirect:
		return ModeNameDirect
	case ModeTLSRF:
		return ModeNameTLSRF
	case ModeTTLD:
		return ModeNameTTLD
	case ModeBlock:
		return ModeNameBlock
	case ModeTLSAlert:
		return ModeNameTLSAlert
	}
	return "unknown"
}

type TriBool uint8

const (
	BoolUnset TriBool = iota
	BoolFalse
	BoolTrue
)

func (b TriBool) IsTrue() bool { return b == BoolTrue }

func (b TriBool) IsUnset() bool { return b == BoolUnset }

func (b *TriBool) UnmarshalJSON(data []byte) error {
	s := string(data)
	switch s {
	case "false":
		*b = BoolFalse
	case "true":
		*b = BoolTrue
	default:
		return E.New("invalid bool: " + s)
	}
	return nil
}

type Byte struct {
	b     byte
	valid bool
}

func (b Byte) IsUnset() bool { return !b.valid }

func (b Byte) Byte() byte { return b.b }

func (b *Byte) Set(x byte) {
	b.valid = true
	b.b = x
}

func (b *Byte) UnmarshalJSON(data []byte) error {
	var x byte
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	b.Set(x)
	return nil
}

type Policy struct {
	ReplyFirst        TriBool
	SniffOverrideMode SniffOverrideMode
	DNSMode           DNSMode
	DNSCacheTTL       time.Duration
	ConnectTimeout    time.Duration
	Host              string
	MapTo             string
	Port              int
	HttpStatus        int
	TLS13Only         TriBool
	Mode              Mode

	NumRecords   int
	NumSegments  int
	WaitForAck   TriBool
	OOB          TriBool
	OOBEx        TriBool
	MinorVer     Byte
	SendInterval time.Duration

	FakeTTL       int
	FakeSleep     time.Duration
	MaxTTL        int
	Attempts      int
	SingleTimeout time.Duration
	TTLCacheTTL   time.Duration
}

func (p *Policy) UnmarshalJSON(data []byte) error {
	var tmp struct {
		SniffOverrideMode SniffOverrideMode `json:"sniff_override"`
		ReplyFirst        TriBool           `json:"reply_first"`
		ConnectTimeout    *string           `json:"connect_timeout"`
		Host              *string           `json:"host"`
		MapTo             *string           `json:"map_to"`
		Port              *uint16           `json:"port"`
		DNSMode           DNSMode           `json:"dns_mode"`
		DNSCacheTTL       *string           `json:"dns_cache_ttl"`
		HttpStatus        *uint             `json:"http_status"`
		TLS13Only         TriBool           `json:"tls13_only"`
		Mode              Mode              `json:"mode"`
		NumRecords        *uint             `json:"num_records"`
		NumSegments       *int              `json:"num_segs"`
		WaitForAck        TriBool           `json:"wait_for_ack"`
		OOB               TriBool           `json:"oob"`
		OOBEx             TriBool           `json:"oob_ex"`
		MinorVer          Byte              `json:"minor_ver"`
		SendInterval      *string           `json:"send_interval"`
		FakeTTL           *uint8            `json:"fake_ttl"`
		FakeSleep         *string           `json:"fake_sleep"`
		MaxTTL            *uint8            `json:"max_ttl"`
		Attempts          *uint             `json:"attempts"`
		SingleTimeout     *string           `json:"single_timeout"`
		TTLCacheTTL       *string           `json:"ttl_cache_ttl"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	p.SniffOverrideMode = tmp.SniffOverrideMode
	p.ReplyFirst = tmp.ReplyFirst
	p.TLS13Only = tmp.TLS13Only
	p.Mode = tmp.Mode
	p.DNSMode = tmp.DNSMode
	p.OOB = tmp.OOB
	p.OOBEx = tmp.OOBEx
	p.MinorVer = tmp.MinorVer
	p.WaitForAck = tmp.WaitForAck

	if tmp.Host == nil {
		p.Host = unsetString
	} else if *tmp.Host == unsetString {
		return E.New("host cannot be `\\x00`")
	} else {
		p.Host = *tmp.Host
	}

	if tmp.MapTo == nil {
		p.MapTo = unsetString
	} else if *tmp.MapTo == unsetString {
		return E.New("map_to cannot be `\\x00`")
	} else {
		p.MapTo = *tmp.MapTo
	}

	if tmp.Port == nil {
		p.Port = unsetInt
	} else {
		p.Port = int(*tmp.Port)
	}

	if tmp.HttpStatus == nil {
		p.HttpStatus = unsetInt
	} else {
		p.HttpStatus = int(*tmp.HttpStatus)
	}

	if tmp.NumRecords != nil {
		if *tmp.NumRecords == 0 {
			return E.New("num_records cannot be 0")
		}
		p.NumRecords = int(*tmp.NumRecords)
	}

	if tmp.NumSegments != nil {
		if *tmp.NumSegments == 0 {
			return E.New("num_segs cannot be 0")
		}
		p.NumSegments = *tmp.NumSegments
	}

	if tmp.FakeTTL == nil {
		p.FakeTTL = unsetInt
	} else {
		p.FakeTTL = int(*tmp.FakeTTL)
	}

	if tmp.Attempts != nil {
		if *tmp.Attempts == 0 {
			return E.New("attempts cannot be 0")
		}
		p.Attempts = int(*tmp.Attempts)
	}

	if tmp.MaxTTL != nil {
		if *tmp.MaxTTL == 0 {
			return E.New("max_ttl cannot be 0")
		}
		p.MaxTTL = int(*tmp.MaxTTL)
	}

	var err error
	if tmp.ConnectTimeout == nil {
		p.ConnectTimeout = unsetInt
	} else {
		p.ConnectTimeout, err = time.ParseDuration(*tmp.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("parse connect_timeout %s: %w", *tmp.ConnectTimeout, err)
		}
		if p.ConnectTimeout <= 0 {
			return fmt.Errorf("connect_timeout %s: must be greater than 0", *tmp.ConnectTimeout)
		}
	}

	if tmp.SendInterval == nil {
		p.SendInterval = unsetInt
	} else {
		p.SendInterval, err = time.ParseDuration(*tmp.SendInterval)
		if err != nil {
			return fmt.Errorf("parse send_interval %s: %w", *tmp.SendInterval, err)
		}
		if p.SendInterval < 0 {
			return fmt.Errorf("send_interval %s: outside the valid range", *tmp.SendInterval)
		}
	}

	if tmp.FakeSleep != nil {
		p.FakeSleep, err = time.ParseDuration(*tmp.FakeSleep)
		if err != nil {
			return fmt.Errorf("parse fake_sleep %s: %w", *tmp.FakeSleep, err)
		}
		if p.FakeSleep <= 0 {
			return fmt.Errorf("fake_sleep %s: must be greater than 0", *tmp.FakeSleep)
		}
	}

	if tmp.SingleTimeout == nil {
		p.SingleTimeout = unsetInt
	} else {
		p.SingleTimeout, err = time.ParseDuration(*tmp.SingleTimeout)
		if err != nil {
			return fmt.Errorf("parse single_timeout %s: %w", *tmp.SingleTimeout, err)
		}
		if p.SingleTimeout <= 0 {
			return fmt.Errorf("single_timeout %s: must be greater than 0", *tmp.SingleTimeout)
		}
	}

	if tmp.DNSCacheTTL == nil {
		p.DNSCacheTTL = unsetInt
	} else {
		p.DNSCacheTTL, err = time.ParseDuration(*tmp.DNSCacheTTL)
		if err != nil {
			return fmt.Errorf("parse dns_cache_ttl %s: %w", *tmp.DNSCacheTTL, err)
		}
		if p.DNSCacheTTL < 0 {
			return fmt.Errorf("dns_cache_ttl %s: must be greater than -1", *tmp.DNSCacheTTL)
		}
	}

	if tmp.TTLCacheTTL == nil {
		p.TTLCacheTTL = unsetInt
	} else {
		p.TTLCacheTTL, err = time.ParseDuration(*tmp.TTLCacheTTL)
		if err != nil {
			return fmt.Errorf("parse ttl_cache_ttl %s: %w", *tmp.TTLCacheTTL, err)
		}
		if p.TTLCacheTTL < 0 {
			return fmt.Errorf("ttl_cache_ttl %s: must be greater than -1", *tmp.TTLCacheTTL)
		}
	}

	return nil
}

func (p Policy) String() string {
	fields := make([]string, 0, 13)
	if p.ConnectTimeout != 0 {
		fields = append(fields, "timeout="+p.ConnectTimeout.String())
	}
	if p.Port != unsetInt && p.Port != 0 {
		fields = append(fields, "port="+F.Int(p.Port))
	}
	if p.Host == "" || p.Host == unsetString {
		if p.DNSMode != DNSModeUnset {
			fields = append(fields, p.DNSMode.String())
		}
		if p.DNSCacheTTL > 0 {
			fields = append(fields, "dns_cache_ttl="+p.DNSCacheTTL.String())
		}
	}
	if p.HttpStatus > 0 {
		fields = append(fields, "http_status="+F.Int(p.HttpStatus))
	}
	if p.TLS13Only.IsTrue() {
		fields = append(fields, "tls13_only")
	}
	fields = append(fields, p.Mode.String())
	switch p.Mode {
	case ModeTLSRF:
		if !p.MinorVer.IsUnset() {
			fields = append(fields, "minor_ver="+F.Uint(p.MinorVer.Byte()))
		}
		if p.NumRecords != unsetInt && p.NumRecords != 1 {
			fields = append(fields, "records="+F.Int(p.NumRecords))
		}
		if p.NumSegments != unsetInt && p.NumSegments != 1 {
			fields = append(fields, "segs="+F.Int(p.NumSegments))
		}
		if p.SendInterval > 0 {
			fields = append(fields, "send_interval="+p.SendInterval.String())
		}
		if p.OOB.IsTrue() {
			fields = append(fields, "oob")
		}
		if p.OOBEx.IsTrue() {
			fields = append(fields, "oob_ex")
		}

	case ModeTTLD:
		if p.FakeTTL == 0 || p.FakeTTL == unsetInt {
			fields = append(fields, "auto_fake_ttl")
			if p.Attempts != 0 {
				fields = append(fields, "attempts="+F.Int(p.Attempts))
			}
			if p.MaxTTL != 0 {
				fields = append(fields, "max_ttl="+F.Int(p.MaxTTL))
			}
			if p.SingleTimeout != 0 {
				fields = append(fields, "single_timeout="+p.SingleTimeout.String())
			}
			if p.TTLCacheTTL > 0 {
				fields = append(fields, "ttl_cache_ttl="+p.TTLCacheTTL.String())
			}
		} else {
			fields = append(fields, "fake_ttl="+F.Int(p.FakeTTL))
		}
		fields = append(fields, "fake_sleep="+p.FakeSleep.String())
	}
	return strings.Join(fields, " ")
}

func mergePolicies(policies ...*Policy) *Policy {
	merged := Policy{
		Host:           unsetString,
		MapTo:          unsetString,
		Port:           unsetInt,
		HttpStatus:     unsetInt,
		SendInterval:   unsetInt,
		FakeTTL:        unsetInt,
		ConnectTimeout: unsetInt,
		SingleTimeout:  unsetInt,
		DNSCacheTTL:    unsetInt,
		TTLCacheTTL:    unsetInt,
	}
	for _, p := range policies {
		if merged.SniffOverrideMode == SniffOverrideUnset && p.SniffOverrideMode != SniffOverrideUnset {
			merged.SniffOverrideMode = p.SniffOverrideMode
		}
		if merged.ReplyFirst.IsUnset() && !p.ReplyFirst.IsUnset() {
			merged.ReplyFirst = p.ReplyFirst
		}
		if merged.ConnectTimeout == unsetInt && p.ConnectTimeout != unsetInt {
			merged.ConnectTimeout = p.ConnectTimeout
		}
		if merged.Host == unsetString && p.Host != unsetString {
			merged.Host = p.Host
		}
		if merged.MapTo == unsetString && p.MapTo != unsetString {
			merged.MapTo = p.MapTo
		}
		if merged.Port == unsetInt && p.Port != unsetInt {
			merged.Port = p.Port
		}
		if merged.HttpStatus == unsetInt && p.HttpStatus != unsetInt {
			merged.HttpStatus = p.HttpStatus
		}
		if merged.TLS13Only.IsUnset() && !p.TLS13Only.IsUnset() {
			merged.TLS13Only = p.TLS13Only
		}
		if merged.Mode == ModeUnset && p.Mode != ModeUnset {
			merged.Mode = p.Mode
		}
		if merged.DNSMode == DNSModeUnset && p.DNSMode != DNSModeUnset {
			merged.DNSMode = p.DNSMode
		}
		if merged.DNSCacheTTL == unsetInt && p.DNSCacheTTL != unsetInt {
			merged.DNSCacheTTL = p.DNSCacheTTL
		}
		if merged.NumRecords == 0 && p.NumRecords != 0 {
			merged.NumRecords = p.NumRecords
		}
		if merged.NumSegments == 0 && p.NumSegments != 0 {
			merged.NumSegments = p.NumSegments
		}
		if merged.WaitForAck.IsUnset() && !p.WaitForAck.IsUnset() {
			merged.WaitForAck = p.WaitForAck
		}
		if merged.OOB.IsUnset() && !p.OOB.IsUnset() {
			merged.OOB = p.OOB
		}
		if merged.OOBEx.IsUnset() && !p.OOBEx.IsUnset() {
			merged.OOBEx = p.OOBEx
		}
		if merged.MinorVer.IsUnset() && !p.MinorVer.IsUnset() {
			merged.MinorVer = p.MinorVer
		}
		if merged.SendInterval == unsetInt && p.SendInterval != unsetInt {
			merged.SendInterval = p.SendInterval
		}
		if merged.FakeSleep == 0 && p.FakeSleep != 0 {
			merged.FakeSleep = p.FakeSleep
		}
		if merged.FakeTTL == unsetInt && p.FakeTTL != unsetInt {
			merged.FakeTTL = p.FakeTTL
		}
		if merged.MaxTTL == 0 && p.MaxTTL != 0 {
			merged.MaxTTL = p.MaxTTL
		}
		if merged.Attempts == 0 && p.Attempts != 0 {
			merged.Attempts = p.Attempts
		}
		if merged.SingleTimeout == unsetInt && p.SingleTimeout != unsetInt {
			merged.SingleTimeout = p.SingleTimeout
		}
		if merged.TTLCacheTTL == unsetInt && p.TTLCacheTTL != unsetInt {
			merged.TTLCacheTTL = p.TTLCacheTTL
		}
	}
	if merged.Mode == ModeUnset {
		merged.Mode = ModeDefault
	}
	if merged.DNSMode == DNSModeUnset {
		merged.DNSMode = DNSModeDefault
	}
	return &merged
}

const (
	noRedirectPrefix = "^"
	ipPoolTagPrefix  = "$"
	resolvePrefix    = "?"
)

func isIPv6(ip string) bool {
	return strings.Contains(ip, ":")
}

func getIPPolicy(ip string) (*Policy, bool) {
	if isIPv6(ip) {
		return ipv6Matcher.Find(ip)
	}
	return ipMatcher.Find(ip)
}

var dohConnPolicy *Policy

type policyConn struct {
	net.Conn
	handled bool
}

func (c *policyConn) Write(b []byte) (n int, err error) {
	if c.handled {
		return c.Conn.Write(b)
	}
	c.handled = true
	var sniStart, sniLen int
	var hasKeyShare bool
	_, sniStart, sniLen, hasKeyShare, _, err = parseClientHello(b)
	if err != nil {
		return
	}
	if dohConnPolicy.TLS13Only.IsTrue() && !hasKeyShare {
		return 0, E.New("not a TLS 1.3 ClientHello")
	}
	if sniStart == -1 {
		return c.Conn.Write(b)
	}
	switch dohConnPolicy.Mode {
	case ModeDirect, ModeRaw:
		return c.Conn.Write(b)
	case ModeTTLD:
		raddr := c.RemoteAddr().String()
		ipv6 := raddr[0] == '['
		ttl, err := getFakeTTL(nil, dohConnPolicy, raddr, ipv6)
		if err != nil {
			return 0, E.WithStr("get fake ttl", err)
		}
		if err = desyncSend(
			c.Conn, ipv6, b,
			sniStart, sniLen, ttl, dohConnPolicy.FakeSleep,
		); err != nil {
			return 0, E.WithStr("ttl desync", err)
		}
	case ModeTLSRF:
		if err = sendRecords(c.Conn, b, sniStart, sniLen,
			dohConnPolicy.NumRecords, dohConnPolicy.NumSegments,
			dohConnPolicy.MinorVer,
			dohConnPolicy.OOB.IsTrue(), dohConnPolicy.OOBEx.IsTrue(),
			dohConnPolicy.WaitForAck.IsTrue(),
			dohConnPolicy.SendInterval); err != nil {
			return 0, E.WithStr("tls fragment", err)
		}
	}
	n = len(b)
	return
}

func genDoHDialFunc() (func(ctx context.Context, network, address string) (net.Conn, error), error) {
	parsedURL, err := url.Parse(dnsAddr)
	if err != nil {
		return nil, E.WithStr("invalid DoH URL", err)
	}
	host := parsedURL.Hostname()
	dohConnPolicy = new(Policy)
	if net.ParseIP(host) != nil {
		var ipPolicy *Policy
		host, ipPolicy, err = ipRedirect(nil, host)
		if ipPolicy == nil {
			dohConnPolicy = &defaultPolicy
		} else {
			dohConnPolicy = mergePolicies(ipPolicy, &defaultPolicy)
		}
		if err != nil {
			return nil, E.WithStr("ip redirect", err)
		}
	} else {
		domainPolicy, foundDomainPolicy := domainMatcher.Find(host)
		if foundDomainPolicy {
			dohConnPolicy = mergePolicies(domainPolicy, &defaultPolicy)
		} else {
			dohConnPolicy = &defaultPolicy
		}
		policyHost := dohConnPolicy.Host
		if strings.HasPrefix(policyHost, noRedirectPrefix) {
			policyHost = policyHost[1:]
		}
		var selectedHost string
		if policyHost == "" || policyHost == unsetString {
			var foundInHosts bool
			selectedHost, foundInHosts = hostsMatcher.Find(host)
			if foundInHosts && strings.HasPrefix(selectedHost, noRedirectPrefix) {
				selectedHost = selectedHost[1:]
			}
		} else {
			selectedHost = policyHost
		}
		switch {
		case selectedHost == "self":
		case strings.HasPrefix(selectedHost, ipPoolTagPrefix):
			if host, err = getFromIPPool(selectedHost[1:]); err != nil {
				return nil, err
			}
		case strings.HasPrefix(selectedHost, resolvePrefix):
		default:
			host = selectedHost
		}
	}
	switch dohConnPolicy.Mode {
	case ModeBlock, ModeTLSAlert:
		return nil, E.New("the mode of the DoH cannot be `block`")
	}
	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}
	if dohConnPolicy.Port != unsetInt {
		port = F.Int(dohConnPolicy.Port)
	}
	addr := net.JoinHostPort(host, port)
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, err := dial.DialTimeout(ctx, network, addr, dohConnPolicy.ConnectTimeout)
		if err == nil {
			return &policyConn{Conn: conn}, nil
		}
		return nil, err
	}, nil
}

func genPolicy(logger log.Logger, originHost string, isIP, returnWhenDomainNotFound bool) (dstHost string, p *Policy, failed, blocked, domainNotFound bool) {
	var err error

	isIP = isIP || net.ParseIP(originHost) != nil
	if isIP {
		var ipPolicy *Policy
		dstHost, ipPolicy, err = ipRedirect(logger, originHost)
		if err != nil {
			logger.Error("IP redirect: ", err)
			return "", nil, true, false, false
		}
		if ipPolicy == nil {
			p = &defaultPolicy
		} else {
			p = mergePolicies(ipPolicy, &defaultPolicy)
		}
		if p.Mode == ModeBlock {
			return "", nil, false, true, false
		}
		return
	}

	domainPolicy, foundDomainPolicy := domainMatcher.Find(originHost)
	if foundDomainPolicy {
		if domainPolicy.Mode == ModeBlock {
			return "", nil, false, true, false
		}
		p = mergePolicies(domainPolicy, &defaultPolicy)
	} else {
		p = &defaultPolicy
	}

	noRedirect := strings.HasPrefix(p.Host, noRedirectPrefix)
	policyHost := p.Host
	if noRedirect {
		policyHost = policyHost[1:]
	}
	var selectedHost string
	var foundInHosts bool
	if policyHost == "" || policyHost == unsetString {
		selectedHost, foundInHosts = hostsMatcher.Find(originHost)
		noRedirect = strings.HasPrefix(selectedHost, noRedirectPrefix)
		if noRedirect {
			selectedHost = selectedHost[1:]
		}
		switch selectedHost {
		case "", unsetString:
			if returnWhenDomainNotFound {
				return "", nil, false, false, true
			}
			var cached bool
			dstHost, cached, err = dnsResolve(originHost, p.DNSMode, p.DNSCacheTTL)
			if err != nil {
				logger.Error("Resolve ", originHost, ": ", err)
				return "", nil, true, false, false
			}
			if cached {
				logger.Info("DNS (cached): ", originHost, " -> ", dstHost)
			} else {
				logger.Info("DNS: ", originHost, " -> ", dstHost)
			}
		}
	} else {
		selectedHost = policyHost
	}

	if dstHost == "" {
		if strings.HasPrefix(selectedHost, resolvePrefix) {
			selectedHost = selectedHost[1:]
			var cached bool
			dstHost, cached, err = dnsResolve(selectedHost, p.DNSMode, p.DNSCacheTTL)
			if err != nil {
				logger.Error("Resolve ", selectedHost, ": ", err)
				return "", nil, true, false, false
			}
			var logPrefix string
			if cached {
				logPrefix = "DNS (cached): "
			} else {
				logPrefix = "DNS: "
			}
			logger.Info(logPrefix, originHost, " -> ", selectedHost, " -> ", dstHost)
		} else {
			var logPrefix string
			if foundInHosts {
				logPrefix = "Host (from hosts): "
			} else {
				logPrefix = "Host: "
			}
			switch {
			case selectedHost == "self":
				dstHost = originHost
				logger.Info(logPrefix, originHost)
			case strings.HasPrefix(selectedHost, ipPoolTagPrefix):
				if dstHost, err = getFromIPPool(selectedHost[1:]); err != nil {
					logger.Error(err)
					return "", nil, true, false, false
				}
				logger.Info(logPrefix, selectedHost, " -> ", dstHost)
			default:
				dstHost = selectedHost
				logger.Info(logPrefix, dstHost)
			}
		}
	}

	if !noRedirect {
		var ipPolicy *Policy
		dstHost, ipPolicy, err = ipRedirect(logger, dstHost)
		if err != nil {
			logger.Info("IP redirect: ", err)
			return "", nil, true, false, false
		}
		if ipPolicy != nil {
			if foundDomainPolicy {
				p = mergePolicies(domainPolicy, ipPolicy, &defaultPolicy)
			} else {
				p = mergePolicies(ipPolicy, &defaultPolicy)
			}
			if p.Mode == ModeBlock {
				return "", nil, false, true, false
			}
		}
	}

	return
}

func ipRedirect(logger log.Logger, ip string) (string, *Policy, error) {
	policy, exists := getIPPolicy(ip)
	if !exists {
		return ip, nil, nil
	}
	if policy.MapTo == "" || policy.MapTo == unsetString {
		return ip, policy, nil
	}
	mapTo := policy.MapTo
	var err error
	if strings.HasPrefix(mapTo, ipPoolTagPrefix) {
		if mapTo, err = getFromIPPool(mapTo[1:]); err != nil {
			return "", nil, err
		}
	} else if strings.LastIndexByte(policy.MapTo, '/') != -1 {
		mapTo, err = transformIP(ip, policy.MapTo)
		if err != nil {
			return "", nil, err
		}
	}
	if logger != nil && ip != mapTo {
		logger.Info("Redirect: ", ip, " -> ", mapTo)
	}
	return mapTo, policy, nil
}

func transformIP(ipStr string, targetNetStr string) (string, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", E.New("invalid IP")
	}

	prefix, err := netip.ParsePrefix(targetNetStr)
	if err != nil {
		return "", E.WithStr("invalid target network", err)
	}

	if ip.Is4() != prefix.Addr().Is4() {
		return "", E.New("IP version mismatch between source IP and target network")
	}

	networkAddr := prefix.Masked().Addr()
	bits := prefix.Bits()

	var newIP netip.Addr
	if ip.Is4() {
		ipBytes := ip.As4()
		netBytes := networkAddr.As4()
		var newBytes [4]byte

		for i := range 4 {
			bitPos := uint8(i * 8)
			if bits >= int(bitPos+8) {
				newBytes[i] = netBytes[i]
			} else if bits <= int(bitPos) {
				newBytes[i] = ipBytes[i]
			} else {
				maskBits := uint8(bits) - bitPos
				mask := uint8(0xFF << (8 - maskBits))
				newBytes[i] = (netBytes[i] & mask) | (ipBytes[i] & ^mask)
			}
		}
		newIP = netip.AddrFrom4(newBytes)
	} else {
		ipBytes := ip.As16()
		netBytes := networkAddr.As16()
		var newBytes [16]byte

		for i := range 16 {
			bitPos := uint8(i * 8)
			if bits >= int(bitPos+8) {
				newBytes[i] = netBytes[i]
			} else if bits <= int(bitPos) {
				newBytes[i] = ipBytes[i]
			} else {
				maskBits := uint8(bits) - bitPos
				mask := uint8(0xFF << (8 - maskBits))
				newBytes[i] = (netBytes[i] & mask) | (ipBytes[i] & ^mask)
			}
		}
		newIP = netip.AddrFrom16(newBytes)
	}

	return newIP.String(), nil
}
