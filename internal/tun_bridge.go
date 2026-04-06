package lumine

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	log "github.com/moi-si/mylog"
)

func NewSessionLogger(prefix string) *log.Logger {
	return newLogger(prefix)
}

func WrapTCPConn(conn net.Conn, plan DialPlan, logger *log.Logger) net.Conn {
	return &policyConn{
		Conn:   conn,
		plan:   plan,
		logger: logger,
	}
}

type policyConn struct {
	net.Conn

	plan    DialPlan
	logger  *log.Logger
	mu      sync.Mutex
	handled bool
	closed  bool
	pending []byte
}

func (c *policyConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.EOF
	}
	if c.handled {
		return c.Conn.Write(b)
	}

	c.pending = append(c.pending, b...)
	if c.tryHandlePendingLocked() {
		return len(b), nil
	}
	return len(b), nil
}

func (c *policyConn) tryHandlePendingLocked() bool {
	if len(c.pending) == 0 {
		return false
	}

	if c.plan.Blocked || c.plan.Policy.Mode == ModeBlock {
		c.logger.Info("Connection blocked:", c.plan.OriginHost)
		c.closed = true
		_ = c.Conn.Close()
		return true
	}

	if c.plan.Policy.Mode == ModeRaw {
		err := c.flushPendingLocked()
		c.closed = err != nil
		return true
	}

	if len(c.pending) < 5 {
		return false
	}

	if c.pending[0] == 0x16 && c.pending[1] == 0x03 {
		recordLen := 5 + int(binary.BigEndian.Uint16(c.pending[3:5]))
		if len(c.pending) < recordLen {
			return false
		}

		record := append([]byte(nil), c.pending[:recordLen]...)
		tail := append([]byte(nil), c.pending[recordLen:]...)
		err := c.handleTLSLocked(record)
		if err != nil {
			c.closed = true
			return true
		}
		c.handled = true
		c.pending = nil
		if len(tail) > 0 {
			_, err = c.Conn.Write(tail)
			if err != nil {
				c.logger.Error("Forward buffered tail:", err)
				c.closed = true
			}
		}
		return true
	}

	c.handled = true
	err := c.flushPendingLocked()
	c.closed = err != nil
	return true
}

func (c *policyConn) flushPendingLocked() error {
	if len(c.pending) == 0 {
		c.handled = true
		return nil
	}
	_, err := c.Conn.Write(c.pending)
	if err != nil {
		c.logger.Error("Forward initial payload:", err)
		return err
	}
	c.handled = true
	c.pending = nil
	return nil
}

func (c *policyConn) handleTLSLocked(record []byte) error {
	prtVer, sniPos, sniLen, hasKeyShare, err := parseClientHello(record)
	if err != nil {
		c.logger.Error("Parse record:", err)
		return c.flushPendingLocked()
	}

	p := c.plan.Policy
	if p.Mode == ModeTLSAlert {
		c.closed = true
		sendTLSAlert(c.logger, c.Conn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
		_ = c.Conn.Close()
		return io.EOF
	}

	if p.TLS13Only == BoolTrue && !hasKeyShare {
		c.logger.Info("Connection blocked: key_share missing from ClientHello")
		c.closed = true
		sendTLSAlert(c.logger, c.Conn, prtVer, tlsAlertProtocolVersion, tlsAlertLevelFatal)
		_ = c.Conn.Close()
		return io.EOF
	}

	if sniPos > 0 && sniLen > 0 {
		sniStr := string(record[sniPos : sniPos+sniLen])
		if domainPolicy, exists := domainMatcher.Find(sniStr); exists {
			sniPolicy := mergePolicies(domainPolicy, &defaultPolicy)
			switch sniPolicy.Mode {
			case ModeBlock:
				c.logger.Info("Connection blocked:", sniStr)
				c.closed = true
				_ = c.Conn.Close()
				return io.EOF
			case ModeTLSAlert:
				c.logger.Info("Connection blocked (TLS alert):", sniStr)
				c.closed = true
				sendTLSAlert(c.logger, c.Conn, prtVer, tlsAlertAccessDenied, tlsAlertLevelFatal)
				_ = c.Conn.Close()
				return io.EOF
			}
		}
	}

	switch p.Mode {
	case ModeDirect:
		_, err = c.Conn.Write(record)
	case ModeTLSRF:
		err = sendRecords(c.Conn, record, sniPos, sniLen,
			p.NumRecords, p.NumSegments,
			p.OOB == BoolTrue, p.OOBEx == BoolTrue,
			p.ModMinorVer == BoolTrue, p.SendInterval)
	case ModeTTLD:
		ipv6 := len(c.plan.TargetHost) > 0 && c.plan.TargetHost[0] == '['
		ttl := p.FakeTTL
		if ttl == 0 || ttl == unsetInt {
			ttl, err = getFakeTTL(c.logger, &p, c.plan.TargetHost, ipv6)
			if err != nil {
				c.logger.Error("get fake TTL:", err)
				return err
			}
		}
		err = desyncSend(c.Conn, ipv6, record, sniPos, sniLen, ttl, p.FakeSleep)
	default:
		_, err = c.Conn.Write(record)
	}

	if err != nil {
		c.logger.Error("Forward TLS record:", err)
		return err
	}
	return nil
}

type directLuminePacketConn struct {
	net.PacketConn
	timeout time.Duration
}

func NewDirectPacketConn(timeout time.Duration) (net.PacketConn, error) {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}
	return &directLuminePacketConn{
		PacketConn: pc,
		timeout:    timeout,
	}, nil
}

func (pc *directLuminePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		return pc.PacketConn.WriteTo(b, udpAddr)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return 0, err
	}
	return pc.PacketConn.WriteTo(b, udpAddr)
}

func DefaultDialTimeout(p Policy) time.Duration {
	if p.ConnectTimeout > 0 {
		return p.ConnectTimeout
	}
	return 10 * time.Second
}
