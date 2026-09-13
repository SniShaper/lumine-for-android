package core

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
	"github.com/lzpls/enimul/internal/freelru"
	"github.com/lzpls/enimul/internal/singleflight"
	"github.com/quic-go/quic-go"
	"golang.org/x/net/proxy"

	"github.com/miekg/dns"
)

type DNSClient interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

var (
	dnsAddr         string
	dnsClient       DNSClient
	httpClient      *http.Client
	dnsExchange     func(req *dns.Msg) (resp *dns.Msg, err error)
	dnsExchanges    []func(req *dns.Msg) (resp *dns.Msg, err error)
	dnsExchangeIdx  atomic.Int64
	dnsCache        *freelru.ShardedLRU[string, string]
	dnsResolveGroup *singleflight.Group[string, string]
	edns0SubnetOpt  *dns.OPT
	lastDNSConfig   DNSConfig
)

// runtimeStateMu 保护上述 DNS 运行时全局变量与 desync.go 中的 TTL 探测全局
// 变量：setDNS/setTTLProbing（配置加载、网络切换重建）与在途会话并发读之间
// 的数据竞争。写侧在局部完成全部校验与构建后，在锁下一次性提交。
var runtimeStateMu sync.RWMutex

type DNSConfig struct {
	Type          string   `json:"type"`
	Addr          string   `json:"addr"`
	Resolvers     []string `json:"resolvers,omitempty"`
	SingleFlight  bool     `json:"singleflight"`
	DisableCache  bool     `json:"disable_cache"`
	CacheCapacity uint32   `json:"cache_capacity"`
	EDNS0Subnet   string   `json:"edns0_subnet"`

	UDPSize       uint16 `json:"udp_size"`
	ClientTimeout string `json:"client_timeout"`
	WaitTimeout   string `json:"wait_timeout"`
	MinRTT        string `json:"min_rtt"`

	DoHSocks5Addr string `json:"doh_socks5_addr"`
}

// exchangeUpstream 返回当前配置的单端点上游交换函数（无锁快照读取）。
func exchangeUpstream(req *dns.Msg) (*dns.Msg, error) {
	runtimeStateMu.RLock()
	single := dnsExchange
	runtimeStateMu.RUnlock()
	if single == nil {
		return nil, E.New("no dns upstream configured")
	}
	return single(req)
}

func exchangeMsg(req *dns.Msg) (resp *dns.Msg, err error) {
	runtimeStateMu.RLock()
	exchanges, single := dnsExchanges, dnsExchange
	runtimeStateMu.RUnlock()
	if len(exchanges) == 0 {
		if single == nil {
			return nil, E.New("no dns upstream configured")
		}
		return single(req)
	}
	start := int(dnsExchangeIdx.Load()) % len(exchanges)
	var lastErr error
	for i := 0; i < len(exchanges); i++ {
		idx := (start + i) % len(exchanges)
		resp, err = exchanges[idx](req)
		if err == nil {
			dnsExchangeIdx.Store(int64(idx))
			return resp, nil
		}
		lastErr = err
	}
	dnsExchangeIdx.Store(0)
	return nil, lastErr
}

func setDNS(c DNSConfig) error {
	if c.Addr == "" {
		return E.New("dns.addr cannot be empty")
	}

	endpoints := []string{c.Addr}
	for _, r := range c.Resolvers {
		r = strings.TrimSpace(r)
		if r != "" && r != c.Addr {
			endpoints = append(endpoints, r)
		}
	}

	var (
		client    DNSClient
		hClient   *http.Client
		exchanges = make([]func(*dns.Msg) (*dns.Msg, error), 0, len(endpoints))
	)

	switch c.Type {
	case "", "udp": // default
		var err error
		cli := dns.Client{}
		if c.UDPSize > 0 {
			cli.UDPSize = c.UDPSize
		}
		if c.ClientTimeout != "" {
			timeout, err := time.ParseDuration(c.ClientTimeout)
			if err != nil {
				return E.WithStr("invalid dns.client_timeout", err)
			}
			if timeout <= 0 {
				return E.New("dns.client_timeout must be greater than 0")
			}
			cli.Timeout = timeout
		}
		var exClient DNSClient = &cli
		if c.WaitTimeout != "" || c.MinRTT != "" {
			var waitTimeout, minRTT time.Duration
			if c.WaitTimeout != "" {
				waitTimeout, err = time.ParseDuration(c.WaitTimeout)
				if err != nil {
					return E.WithStr("invalid dns.wait_timeout", err)
				}
				if waitTimeout <= 0 {
					return E.New("dns.wait_timeout must be greater than 0")
				}
			}
			if c.MinRTT != "" {
				minRTT, err = time.ParseDuration(c.MinRTT)
				if err != nil {
					return E.WithStr("invalid dns.min_rtt", err)
				}
				if minRTT <= 0 {
					return E.New("dns.min_rtt must be greater than 0")
				}
			}
			exClient = &antiHijackDNSClient{
				Client:      cli,
				waitTimeout: waitTimeout,
				minRTT:      minRTT,
			}
		}
		client = exClient
		for _, ep := range endpoints {
			if _, err := netip.ParseAddrPort(ep); err != nil {
				return E.WithStr("invalid dns.addr", err)
			}
			addr := ep
			exchanges = append(exchanges, func(req *dns.Msg) (*dns.Msg, error) {
				resp, _, err := exClient.Exchange(req, addr)
				return resp, err
			})
		}
	case "tcp":
		cli := &dns.Client{Net: "tcp"}
		client = cli
		for _, ep := range endpoints {
			if _, err := netip.ParseAddrPort(ep); err != nil {
				return E.WithStr("invalid dns.addr", err)
			}
			addr := ep
			exchanges = append(exchanges, func(req *dns.Msg) (*dns.Msg, error) {
				resp, _, err := cli.Exchange(req, addr)
				return resp, err
			})
		}
	case "tls":
		cli := &dns.Client{Net: "tcp-tls"}
		client = cli
		for _, ep := range endpoints {
			if _, err := netip.ParseAddrPort(ep); err != nil {
				return E.WithStr("invalid dns.addr", err)
			}
			addr := ep
			exchanges = append(exchanges, func(req *dns.Msg) (*dns.Msg, error) {
				resp, _, err := cli.Exchange(req, addr)
				return resp, err
			})
		}
	case "https":
		if !isValidHTTPSURL(c.Addr) {
			return E.New("invalid dns.addr")
		}
		for _, ep := range endpoints {
			if !isValidHTTPSURL(ep) {
				return E.New("invalid dns resolver url: " + ep)
			}
			transport := http.DefaultTransport.(*http.Transport).Clone()
			if c.DoHSocks5Addr == "" {
				dial, err := genDoHDialFuncFor(ep)
				if err != nil {
					return E.WithStr("generate DoH dial function", err)
				}
				transport.DialContext = dial
			} else {
				dialer, err := proxy.SOCKS5("tcp", c.DoHSocks5Addr, nil, proxy.Direct)
				if err != nil {
					return E.WithStr("create socks5 dialer", err)
				}
				transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				}
			}
			hc := &http.Client{Transport: transport}
			endpoint := ep
			exchanges = append(exchanges, func(req *dns.Msg) (*dns.Msg, error) {
				wire, err := req.Pack()
				if err != nil {
					return nil, E.WithStr("pack dns request", err)
				}
				queryURL := endpoint + "?dns=" + base64.RawURLEncoding.EncodeToString(wire)
				httpReq, err := http.NewRequest(http.MethodGet, queryURL, nil)
				if err != nil {
					return nil, E.WithStr("build http request", err)
				}
				httpReq.Header.Set("Accept", "application/dns-message")
				httpResp, err := hc.Do(httpReq)
				if err != nil {
					return nil, E.WithStr("http request", err)
				}
				defer httpResp.Body.Close()
				if httpResp.StatusCode != http.StatusOK {
					return nil, E.New("bad http status: " + httpResp.Status)
				}
				respWire, err := io.ReadAll(httpResp.Body)
				if err != nil {
					return nil, E.WithStr("read http body", err)
				}
				resp := new(dns.Msg)
				if err = resp.Unpack(respWire); err != nil {
					return nil, E.WithStr("unpack dns response", err)
				}
				return resp, nil
			})
		}
		hClient = &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	case "quic":
		for _, ep := range endpoints {
			if _, _, err := net.SplitHostPort(ep); err != nil {
				ep = net.JoinHostPort(ep, "853")
			}
			dq := newDoQClient(ep)
			exchanges = append(exchanges, func(req *dns.Msg) (*dns.Msg, error) {
				return dq.exchange(req)
			})
		}
	default:
		return E.NewAny("unknown dns.type: ", c.Type)
	}

	var group *singleflight.Group[string, string]
	if c.SingleFlight {
		group = new(singleflight.Group[string, string])
	}

	var cache, ipCache *freelru.ShardedLRU[string, string]
	if !c.DisableCache {
		capacity := c.CacheCapacity
		if capacity == 0 {
			capacity = 4096
		}
		var err error
		if cache, err = freelru.NewSharded[string, string](capacity, hashStringXXHASH); err != nil {
			return E.WithStr("init DNS cache", err)
		}
		if ipCache, err = freelru.NewSharded[string, string](capacity, hashStringXXHASH); err != nil {
			return E.WithStr("init ip-domain cache", err)
		}
	}

	var edns0 *dns.OPT
	if c.EDNS0Subnet != "" {
		prefix, err := netip.ParsePrefix(c.EDNS0Subnet)
		if err != nil {
			return fmt.Errorf("invalid edns0_subnet %s: %w", c.EDNS0Subnet, err)
		}
		family := uint16(1)
		if prefix.Addr().Unmap().Is6() {
			family = 2
		}
		edns0Opt := &dns.EDNS0_SUBNET{
			Code:          dns.EDNS0SUBNET,
			Family:        family,
			SourceNetmask: uint8(prefix.Bits()),
			Address:       prefix.Addr().AsSlice(),
		}
		edns0 = &dns.OPT{
			Hdr:    dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT},
			Option: []dns.EDNS0{edns0Opt},
		}
	}

	// 全部端点校验与构建完成，锁下一次性提交，避免半提交的不一致状态。
	runtimeStateMu.Lock()
	lastDNSConfig = c
	dnsAddr = c.Addr
	dnsClient = client
	httpClient = hClient
	dnsExchanges = exchanges
	if len(exchanges) > 0 {
		dnsExchange = exchanges[0]
	} else {
		dnsExchange = nil
	}
	dnsResolveGroup = group
	dnsCache = cache
	ipDomainCache = ipCache
	edns0SubnetOpt = edns0
	runtimeStateMu.Unlock()

	return nil
}

func isValidHTTPSURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

type DNSMode uint8

const (
	DNSModeUnset DNSMode = iota
	DNSModePreferIPv4
	DNSModePreferIPv6
	DNSModeIPv4Only
	DNSModeIPv6Only
	DNSModeDefault = DNSModePreferIPv4
)

const (
	DNSModeNamePreferIPv4 = "prefer_ipv4"
	DNSModeNamePreferIPv6 = "prefer_ipv6"
	DNSModeNameIPv4Only   = "ipv4_only"
	DNSModeNameIPv6Only   = "ipv6_only"
)

func (m DNSMode) String() string {
	switch m {
	case DNSModePreferIPv4:
		return DNSModeNamePreferIPv4
	case DNSModePreferIPv6:
		return DNSModeNamePreferIPv6
	case DNSModeIPv4Only:
		return DNSModeNameIPv4Only
	case DNSModeIPv6Only:
		return DNSModeNameIPv6Only
	}
	return "unknown"
}

func (m *DNSMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case DNSModeNamePreferIPv4:
		*m = DNSModePreferIPv4
	case DNSModeNamePreferIPv6:
		*m = DNSModePreferIPv6
	case DNSModeNameIPv4Only:
		*m = DNSModeIPv4Only
	case DNSModeNameIPv6Only:
		*m = DNSModeIPv6Only
	default:
		return E.New("invalid dns_mode: " + s)
	}
	return nil
}

func dnsClientExchange(req *dns.Msg) (resp *dns.Msg, err error) {
	runtimeStateMu.RLock()
	client, addr := dnsClient, dnsAddr
	runtimeStateMu.RUnlock()
	if client == nil {
		return nil, E.New("no dns client configured")
	}
	resp, _, err = client.Exchange(req, addr)
	return resp, err
}

func dohExchange(req *dns.Msg) (resp *dns.Msg, err error) {
	runtimeStateMu.RLock()
	addr, hc := dnsAddr, httpClient
	runtimeStateMu.RUnlock()
	if hc == nil {
		return nil, E.New("no doh client configured")
	}
	wire, err := req.Pack()
	if err != nil {
		return nil, E.WithStr("pack dns request", err)
	}
	url := addr + "?dns=" + base64.RawURLEncoding.EncodeToString(wire)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, E.WithStr("build http request", err)
	}
	httpReq.Header.Set("Accept", "application/dns-message")
	httpResp, err := hc.Do(httpReq)
	if err != nil {
		return nil, E.WithStr("http request", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, E.New("bad http status: " + httpResp.Status)
	}
	respWire, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, E.WithStr("read http body", err)
	}
	resp = new(dns.Msg)
	if err = resp.Unpack(respWire); err != nil {
		return nil, E.WithStr("unpack dns response", err)
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

func doDNSResolve(domain string, mode DNSMode, cacheTTL time.Duration) (string, error) {
	msg := new(dns.Msg)
	switch mode {
	case DNSModePreferIPv4, DNSModeIPv4Only:
		msg.SetQuestion(domain+".", dns.TypeA)
	case DNSModePreferIPv6, DNSModeIPv6Only:
		msg.SetQuestion(domain+".", dns.TypeAAAA)
	}
	runtimeStateMu.RLock()
	edns0, cache := edns0SubnetOpt, dnsCache
	runtimeStateMu.RUnlock()
	if edns0 != nil {
		msg.Extra = []dns.RR{edns0}
	}

	resp, err := exchangeMsg(msg)
	if err != nil {
		return "", E.WithStr("dns exchange", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		return "", E.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
	}

	var ip net.IP
	switch mode {
	case DNSModeIPv4Only:
		if ip = pickFirstARecord(resp.Answer); ip == nil {
			return "", E.New("A record not found")
		}
	case DNSModeIPv6Only:
		if ip = pickFirstAAAARecord(resp.Answer); ip == nil {
			return "", E.New("AAAA record not found")
		}
	case DNSModePreferIPv4:
		if ip = pickFirstARecord(resp.Answer); ip == nil {
			msg.SetQuestion(domain+".", dns.TypeAAAA)
			resp, err2 := exchangeMsg(msg)
			if err2 != nil {
				return "", E.WithStr("dns exchange", E.Join(err, err2))
			}
			if resp.Rcode != dns.RcodeSuccess {
				return "", E.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
			}
			if ip = pickFirstAAAARecord(resp.Answer); ip == nil {
				return "", E.New("record not found")
			}
		}
	case DNSModePreferIPv6:
		if ip = pickFirstAAAARecord(resp.Answer); ip == nil {
			msg.SetQuestion(domain+".", dns.TypeA)
			resp, err2 := exchangeMsg(msg)
			if err2 != nil {
				return "", E.WithStr("dns exchange", E.Join(err, err2))
			}
			if resp.Rcode != dns.RcodeSuccess {
				return "", E.New("bad rcode: " + dns.RcodeToString[resp.Rcode])
			}
			if ip = pickFirstARecord(resp.Answer); ip == nil {
				return "", E.New("record not found")
			}
		}
	}

	ipStr := ip.String()
	if cacheTTL != 0 && cacheTTL != unsetInt && cache != nil {
		cache.AddWithLifetime(domain, ipStr, cacheTTL)
	}
	return ipStr, nil
}

func dnsResolve(domain string, mode DNSMode, cacheTTL time.Duration) (ip string, cached bool, err error) {
	runtimeStateMu.RLock()
	cache, group := dnsCache, dnsResolveGroup
	runtimeStateMu.RUnlock()
	if cache != nil {
		if ip, ok := cache.Get(domain); ok {
			return ip, true, nil
		}
	}

	if group == nil {
		ip, err = doDNSResolve(domain, mode, cacheTTL)
	} else {
		ip, err, _ = group.Do(domain, func() (string, error) {
			return doDNSResolve(domain, mode, cacheTTL)
		})
	}

	return
}

// Modified from github.com/miekg/dns.Client
type antiHijackDNSClient struct {
	dns.Client
	waitTimeout, minRTT time.Duration
}

const dnsRWTimeout = 2 * time.Second

func (c *antiHijackDNSClient) readTimeout() time.Duration {
	if c.Timeout != 0 {
		return c.Timeout
	}
	if c.ReadTimeout != 0 {
		return c.ReadTimeout
	}
	return dnsRWTimeout
}

func (c *antiHijackDNSClient) writeTimeout() time.Duration {
	if c.Timeout != 0 {
		return c.Timeout
	}
	if c.WriteTimeout != 0 {
		return c.WriteTimeout
	}
	return dnsRWTimeout
}

func (c *antiHijackDNSClient) getTimeoutForRequest(timeout time.Duration) time.Duration {
	var requestTimeout time.Duration
	if c.Timeout != 0 {
		requestTimeout = c.Timeout
	} else {
		requestTimeout = timeout
	}
	if c.Dialer != nil && c.Dialer.Timeout != 0 {
		if c.Dialer.Timeout < requestTimeout {
			requestTimeout = c.Dialer.Timeout
		}
	}
	return requestTimeout
}

func (c *antiHijackDNSClient) Exchange(m *dns.Msg, address string) (r *dns.Msg, rtt time.Duration, err error) {
	co, err := c.Dial(address)
	if err != nil {
		return nil, 0, err
	}
	defer co.Close()
	return c.ExchangeWithConn(m, co)
}

func (c *antiHijackDNSClient) ExchangeWithConn(m *dns.Msg, conn *dns.Conn) (r *dns.Msg, rtt time.Duration, err error) {
	return c.ExchangeWithConnContext(context.Background(), m, conn)
}

func (c *antiHijackDNSClient) ExchangeWithConnContext(ctx context.Context, m *dns.Msg, co *dns.Conn) (r *dns.Msg, rtt time.Duration, err error) {
	opt := m.IsEdns0()
	if opt != nil && opt.UDPSize() >= dns.MinMsgSize {
		co.UDPSize = opt.UDPSize()
	}
	if opt == nil && c.UDPSize >= dns.MinMsgSize {
		co.UDPSize = c.UDPSize
	}

	t := time.Now()
	writeDeadline := t.Add(c.getTimeoutForRequest(c.writeTimeout()))
	readDeadline := t.Add(c.getTimeoutForRequest(c.readTimeout()))

	if deadline, ok := ctx.Deadline(); ok && !deadline.IsZero() {
		if deadline.Before(writeDeadline) {
			writeDeadline = deadline
		}
		if deadline.Before(readDeadline) {
			readDeadline = deadline
		}
	}
	co.SetWriteDeadline(writeDeadline)

	if c.waitTimeout != 0 {
		if waitDeadline := t.Add(c.waitTimeout); waitDeadline.Before(readDeadline) {
			readDeadline = waitDeadline
		}
	}
	co.SetReadDeadline(readDeadline)

	co.TsigSecret, co.TsigProvider = c.TsigSecret, c.TsigProvider

	if err = co.WriteMsg(m); err != nil {
		return nil, 0, err
	}

	var (
		bestR           *dns.Msg
		bestRecordCount int
		bestRTT         time.Duration
		lastErr         error
	)

	for {
		r, err = co.ReadMsg()
		curRTT := time.Since(t)

		if err != nil {
			lastErr = err
			break
		}

		if c.minRTT != 0 && curRTT < c.minRTT {
			continue
		}

		if r.Id == m.Id {
			runtimeStateMu.RLock()
			edns0 := edns0SubnetOpt
			runtimeStateMu.RUnlock()
			if c.waitTimeout <= 0 || (edns0 != nil && hasEDNS0Subnet(r)) {
				return r, curRTT, nil
			}

			recordCount := len(r.Answer) + len(r.Ns) + len(r.Extra)
			if recordCount >= bestRecordCount {
				bestR = r
				bestRecordCount = recordCount
				bestRTT = curRTT
			}
		}

		if c.waitTimeout > 0 && curRTT >= c.waitTimeout {
			break
		}
	}

	if bestR != nil {
		return bestR, bestRTT, nil
	}

	return r, time.Since(t), lastErr
}

func hasEDNS0Subnet(resp *dns.Msg) bool {
	for _, rr := range resp.Extra {
		opt, ok := rr.(*dns.OPT)
		if !ok {
			continue
		}
		for _, o := range opt.Option {
			if o.Option() == dns.EDNS0SUBNET {
				return true
			}
		}
	}
	return false
}

// doQClient 按 RFC 9250 实现 DoQ：每条查询独占一条 QUIC 流，消息不带长度前缀，
// 写后关闭写侧，读到对端 FIN 即视为响应结束。连接在端点复用，出错时重连一次。
type doQClient struct {
	endpoint string
	server   string
	timeout  time.Duration
	mu       sync.Mutex
	conn     *quic.Conn
}

func newDoQClient(endpoint string) *doQClient {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
	}
	return &doQClient{
		endpoint: endpoint,
		server:   host,
		timeout:  6 * time.Second,
	}
}

func (q *doQClient) dial(ctx context.Context) (*quic.Conn, error) {
	tlsConf := &tls.Config{
		ServerName: q.server,
		NextProtos: []string{"doq"},
		MinVersion: tls.VersionTLS13,
	}
	conf := &quic.Config{HandshakeIdleTimeout: q.timeout}
	return quic.DialAddr(ctx, q.endpoint, tlsConf, conf)
}

func (q *doQClient) exchange(req *dns.Msg) (resp *dns.Msg, err error) {
	wire, err := req.Pack()
	if err != nil {
		return nil, E.WithStr("pack dns request", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	defer cancel()

	if q.conn == nil {
		q.conn, err = q.dial(ctx)
		if err != nil {
			return nil, E.WithStr("quic dial", err)
		}
	}

	stream, err := q.conn.OpenStreamSync(ctx)
	if err != nil {
		_ = q.conn.CloseWithError(0, "stream open failed")
		q.conn = nil
		q.conn, err = q.dial(ctx)
		if err != nil {
			return nil, E.WithStr("quic redial", err)
		}
		stream, err = q.conn.OpenStreamSync(ctx)
		if err != nil {
			return nil, E.WithStr("quic open stream", err)
		}
	}

	if _, err = stream.Write(wire); err != nil {
		_ = q.conn.CloseWithError(0, "write failed")
		q.conn = nil
		return nil, E.WithStr("quic write", err)
	}
	if err = stream.Close(); err != nil {
		return nil, E.WithStr("quic close write", err)
	}
	respWire, err := io.ReadAll(stream)
	if err != nil {
		_ = q.conn.CloseWithError(0, "read failed")
		q.conn = nil
		return nil, E.WithStr("quic read", err)
	}
	resp = new(dns.Msg)
	if err = resp.Unpack(respWire); err != nil {
		return nil, E.WithStr("unpack dns response", err)
	}
	return resp, nil
}
