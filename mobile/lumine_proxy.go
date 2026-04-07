package mobile

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	lumine "github.com/moi-si/lumine/internal"
	log "github.com/moi-si/mylog"
	"github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy/proto"
)

type LumineProxy struct{}

var (
	tcpDialID uint32
	udpDialID uint32
)

func (p *LumineProxy) Addr() string {
	return "lumine"
}

func (p *LumineProxy) Proto() proto.Proto {
	return proto.Direct
}

func (p *LumineProxy) DialContext(ctx context.Context, m *metadata.Metadata) (net.Conn, error) {
	if m == nil || !m.DstIP.IsValid() {
		return nil, fmt.Errorf("invalid tcp metadata: %+v", m)
	}

	originHost := m.DstIP.String()
	logger := lumine.NewSessionLogger(fmt.Sprintf("[T%05x]", nextID(&tcpDialID)))
	if lumine.IsVPNDNSAddress(originHost, int(m.DstPort)) {
		logger.Debug("Hijacking TCP DNS for", net.JoinHostPort(originHost, fmt.Sprintf("%d", m.DstPort)))
		return newLocalDNSTCPConn(logger)
	}

	plan, err := lumine.PlanRequest(lumine.RequestContext{
		Source: lumine.RequestSourceMobile,
		Host:   originHost,
		Port:   int(m.DstPort),
	}, logger)
	if err != nil {
		logger.Error("Build dial plan:", err)
		return nil, err
	}
	if plan.Blocked {
		logger.Info("Connection blocked:", originHost)
		return nil, fmt.Errorf("blocked by policy: %s", originHost)
	}

	target := plan.TargetAddress()
	dialer := &net.Dialer{Timeout: lumine.DefaultDialTimeout(plan.Policy)}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		logger.Error("Connection failed:", err)
		return nil, err
	}

	logDialPlan(logger, "TCP", plan, target)
	switch plan.Policy.Mode {
	case lumine.ModeTTLD:
		logger.Info("Downgrading ttl-d to raw in direct bridge for stability")
		return conn, nil
	default:
		return lumine.WrapTCPConn(conn, plan, logger), nil
	}
}

func (p *LumineProxy) DialUDP(m *metadata.Metadata) (net.PacketConn, error) {
	logger := lumine.NewSessionLogger(fmt.Sprintf("[U%05x]", nextID(&udpDialID)))
	if m == nil || !m.DstIP.IsValid() {
		logger.Error("Invalid UDP metadata:", m)
		return nil, fmt.Errorf("invalid udp metadata: %+v", m)
	}
	pc, err := lumine.NewDirectPacketConn(30 * time.Second)
	if err != nil {
		logger.Error("Open UDP packet conn:", err)
		return nil, err
	}
	logger.Debug("UDP relay ready for", m.DestinationAddress())
	return &luminePacketConn{
		PacketConn: newQueuedPacketConn(pc),
		logger:     logger,
	}, nil
}

type luminePacketConn struct {
	net.PacketConn
	logger *log.Logger
}

func (pc *luminePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		var err error
		udpAddr, err = net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return 0, err
		}
	}

	dstIP := udpAddr.IP
	if addrPort, err := netip.ParseAddrPort(addr.String()); err == nil {
		dstIP = net.IP(addrPort.Addr().AsSlice())
	}
	if len(dstIP) == 0 {
		return 0, fmt.Errorf("invalid udp destination: %v", addr)
	}

	if lumine.IsVPNDNSAddress(dstIP.String(), udpAddr.Port) {
		return pc.handleDNSQuery(b, udpAddr)
	}

	originHost := dstIP.String()
	plan, err := lumine.PlanRequest(lumine.RequestContext{
		Source: lumine.RequestSourceMobile,
		Host:   originHost,
		Port:   udpAddr.Port,
	}, pc.logger)
	if err != nil {
		return 0, err
	}
	if plan.Blocked {
		return 0, fmt.Errorf("blocked by policy: %s", originHost)
	}

	dst, err := net.ResolveUDPAddr("udp", plan.TargetAddress())
	if err != nil {
		return 0, err
	}
	logDialPlan(pc.logger, "UDP", plan, dst.String())
	return pc.PacketConn.WriteTo(b, dst)
}

func (pc *luminePacketConn) handleDNSQuery(payload []byte, addr *net.UDPAddr) (int, error) {
	pc.logger.Debug("DNS hijack recv:", summarizeDNSPacket(payload))
	resp, err := lumine.HandleDNSQueryPacket(payload)
	if err != nil {
		pc.logger.Error("Handle hijacked DNS query:", err)
		return 0, err
	}

	queue, ok := pc.PacketConn.(*queuedPacketConn)
	if !ok {
		return 0, fmt.Errorf("queued packet conn unavailable")
	}
	if err := queue.EnqueuePacket(resp, addr); err != nil {
		return 0, err
	}

	pc.logger.Debug("DNS hijack resp:", summarizeDNSPacket(resp), "to", addr.String())
	return len(payload), nil
}

func logDialPlan(logger *log.Logger, network string, plan lumine.DialPlan, target string) {
	if !plan.MatchedDomain && !plan.MatchedIP && !plan.Blocked {
		return
	}

	message := joinPlanLogMessage(network, plan, target)
	switch plan.Policy.Mode {
	case lumine.ModeDirect, lumine.ModeRaw:
		logger.Debug(message)
	default:
		logger.Info(message)
	}
}

func joinPlanLogMessage(network string, plan lumine.DialPlan, target string) string {
	origin := plan.OriginAddress()
	if plan.RecoveredDomain != "" {
		origin = plan.RecoveredDomain
		if plan.OriginPort > 0 {
			origin = net.JoinHostPort(origin, fmt.Sprintf("%d", plan.OriginPort))
		}
	}
	return fmt.Sprintf("%s %s -> %s mode=%s", network, origin, target, plan.Policy.Mode)
}

func nextID(counter *uint32) uint32 {
	id := atomic.AddUint32(counter, 1)
	if id > 0xFFFFF {
		atomic.StoreUint32(counter, 0)
		return 0
	}
	return id
}

func summarizeDNSPacket(payload []byte) string {
	msg := new(dns.Msg)
	if err := msg.Unpack(payload); err != nil {
		return fmt.Sprintf("unpack_failed bytes=%d err=%v", len(payload), err)
	}
	if len(msg.Question) == 0 {
		return fmt.Sprintf(
			"id=%d questions=0 answers=%d rcode=%s%s",
			msg.Id,
			len(msg.Answer),
			dns.RcodeToString[msg.Rcode],
			summarizeDNSAnswers(msg.Answer),
		)
	}

	q := msg.Question[0]
	qtype := dns.TypeToString[q.Qtype]
	if qtype == "" {
		qtype = fmt.Sprintf("TYPE%d", q.Qtype)
	}
	return fmt.Sprintf(
		"id=%d q=%s type=%s answers=%d rcode=%s%s",
		msg.Id,
		q.Name,
		qtype,
		len(msg.Answer),
		dns.RcodeToString[msg.Rcode],
		summarizeDNSAnswers(msg.Answer),
	)
}

func summarizeDNSAnswers(answers []dns.RR) string {
	if len(answers) == 0 {
		return ""
	}

	limit := len(answers)
	if limit > 3 {
		limit = 3
	}

	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		switch rr := answers[i].(type) {
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
			rrType := dns.TypeToString[answers[i].Header().Rrtype]
			if rrType == "" {
				rrType = fmt.Sprintf("TYPE%d", answers[i].Header().Rrtype)
			}
			parts = append(parts, rrType)
		}
	}

	summary := " [" + joinAnswerParts(parts) + "]"
	if len(answers) > limit {
		summary += fmt.Sprintf(" +%d", len(answers)-limit)
	}
	return summary
}

func joinAnswerParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
