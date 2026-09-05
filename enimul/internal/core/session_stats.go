package core

import (
	"encoding/json"
	"net"
	"sync/atomic"
	"time"
)

type sessionStatsState struct {
	down     atomic.Uint64
	up       atomic.Uint64
	blocked  atomic.Uint64
	tcpConns atomic.Uint64
	udpConns atomic.Uint64
	started  atomic.Int64
}

var sessionStats sessionStatsState

// StatsSnapshot 描述自上次 ResetStats 以来的累计会话指标，
// 语义与桌面版 ProxyServer.GetStats 对齐（Down/Up + 附加计数）。
type StatsSnapshot struct {
	Down     uint64 `json:"down"`
	Up       uint64 `json:"up"`
	Blocked  uint64 `json:"blocked"`
	TCPConns uint64 `json:"tcp_conns"`
	UDPConns uint64 `json:"udp_conns"`
	UptimeMs int64  `json:"uptime_ms"`
}

// ResetStats 清零全部会话计数并把起点设为当前时刻。
func ResetStats() {
	sessionStats.down.Store(0)
	sessionStats.up.Store(0)
	sessionStats.blocked.Store(0)
	sessionStats.tcpConns.Store(0)
	sessionStats.udpConns.Store(0)
	sessionStats.started.Store(time.Now().UnixMilli())
}

func AddRxBytes(n int) {
	if n > 0 {
		sessionStats.down.Add(uint64(n))
	}
}

func AddTxBytes(n int) {
	if n > 0 {
		sessionStats.up.Add(uint64(n))
	}
}

func AddBlockedConn() {
	sessionStats.blocked.Add(1)
}

func IncTCPConn() {
	sessionStats.tcpConns.Add(1)
}

func IncUDPConn() {
	sessionStats.udpConns.Add(1)
}

func SnapshotStats() StatsSnapshot {
	started := sessionStats.started.Load()
	uptime := int64(0)
	if started > 0 {
		uptime = time.Now().UnixMilli() - started
	}
	return StatsSnapshot{
		Down:     sessionStats.down.Load(),
		Up:       sessionStats.up.Load(),
		Blocked:  sessionStats.blocked.Load(),
		TCPConns: sessionStats.tcpConns.Load(),
		UDPConns: sessionStats.udpConns.Load(),
		UptimeMs: uptime,
	}
}

func (s StatsSnapshot) JSON() string {
	data, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// sessionConn 统计实际在物理链路上收发的应用字节。
type sessionConn struct {
	net.Conn
}

func (c *sessionConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	AddRxBytes(n)
	return n, err
}

func (c *sessionConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	AddTxBytes(n)
	return n, err
}

func NewSessionConn(conn net.Conn) net.Conn {
	return &sessionConn{Conn: conn}
}

// sessionPacketConn 统计 UDP 方向字节。
type sessionPacketConn struct {
	net.PacketConn
}

func (c *sessionPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	AddRxBytes(n)
	return n, addr, err
}

func (c *sessionPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	n, err := c.PacketConn.WriteTo(p, addr)
	AddTxBytes(n)
	return n, err
}

func NewSessionPacketConn(pc net.PacketConn) net.PacketConn {
	return &sessionPacketConn{PacketConn: pc}
}
