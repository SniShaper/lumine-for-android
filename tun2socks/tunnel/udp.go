package tunnel

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/xjasonlyu/tun2socks/v2/buffer"
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
	"github.com/xjasonlyu/tun2socks/v2/log"
	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/tunnel/statistic"
)

// TODO: Port Restricted NAT support.
func (t *Tunnel) handleUDPConn(uc adapter.UDPConn) {
	defer uc.Close()

	id := uc.ID()
	metadata := &M.Metadata{
		Network: M.UDP,
		SrcIP:   parseTCPIPAddress(id.RemoteAddress),
		SrcPort: id.RemotePort,
		DstIP:   parseTCPIPAddress(id.LocalAddress),
		DstPort: id.LocalPort,
	}

	pc, err := t.Dialer().DialUDP(metadata)
	if err != nil {
		log.Warnf("[UDP] dial %s: %v", metadata.DestinationAddress(), err)
		return
	}
	metadata.MidIP, metadata.MidPort = parseNetAddr(pc.LocalAddr())

	pc = statistic.NewUDPTracker(pc, metadata, t.manager)
	defer pc.Close()

	var remote net.Addr
	if udpAddr := metadata.UDPAddr(); udpAddr != nil {
		remote = udpAddr
	} else {
		remote = metadata.Addr()
	}
	pc = newSymmetricNATPacketConn(pc, metadata)

	log.Infof("[UDP] %s <-> %s", metadata.SourceAddress(), metadata.DestinationAddress())
	pipePacket(uc, pc, remote, t.udpTimeout.Load())
}

// udpOriginIdleCap bounds how long remote-only traffic can keep a session
// alive: when the origin side (tun) has been silent for more than
// udpOriginIdleCap times the session timeout, the session is torn down
// regardless of remote activity (decision D2).
const udpOriginIdleCap = 3

var errUDPOriginIdleExceeded = errors.New("origin direction idle exceeded bounded renewal cap")

// udpRelayState tracks the last activity timestamp of each direction so
// that cross-direction renewal stays bounded and race-free.
type udpRelayState struct {
	lastOrigin *atomic.Int64 // unix nanos of the last origin (tun side) packet
	lastRemote *atomic.Int64 // unix nanos of the last remote packet
}

func pipePacket(origin, remote net.PacketConn, to net.Addr, timeout time.Duration) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	now := time.Now().UnixNano()
	state := &udpRelayState{
		lastOrigin: atomic.NewInt64(now),
		lastRemote: atomic.NewInt64(now),
	}

	// origin -> remote: bounded idle on the origin side; its exit tears
	// down both ends so that remote-only traffic cannot keep the peer
	// direction (and the session) alive forever.
	go func() {
		defer wg.Done()
		defer origin.Close()
		defer remote.Close()
		if err := copyPacketData(remote, origin, to, timeout, state, true); err != nil {
			log.Debugf("[UDP] copy data for %s: %v", "origin->remote", err)
		}
	}()

	// remote -> origin: no hard cap, but it only survives while the
	// origin side stays active (see copyPacketData).
	go func() {
		defer wg.Done()
		if err := copyPacketData(origin, remote, nil, timeout, state, false); err != nil {
			log.Debugf("[UDP] copy data for %s: %v", "remote->origin", err)
		}
	}()

	wg.Wait()
}

func copyPacketData(dst, src net.PacketConn, to net.Addr, timeout time.Duration, state *udpRelayState, originDir bool) error {
	buf := buffer.Get(buffer.MaxSegmentSize)
	defer buffer.Put(buf)

	timeoutNs := int64(timeout)
	idleCapNs := timeoutNs * udpOriginIdleCap

	var selfLast, peerLast *atomic.Int64
	if originDir {
		selfLast, peerLast = state.lastOrigin, state.lastRemote
	} else {
		selfLast, peerLast = state.lastRemote, state.lastOrigin
	}

	for {
		deadline := time.Now().Add(timeout)
		if originDir {
			// The origin direction never waits past the hard idle cap.
			if hard := time.Unix(0, state.lastOrigin.Load()+idleCapNs); hard.Before(deadline) {
				deadline = hard
			}
		}
		src.SetReadDeadline(deadline)
		n, _, err := src.ReadFrom(buf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			now := time.Now().UnixNano()
			if originDir && now >= state.lastOrigin.Load()+idleCapNs {
				return errUDPOriginIdleExceeded
			}
			// Bounded cross-direction renewal (slow-reply compatibility):
			// keep waiting only while the peer direction had traffic
			// within the last timeout window.
			if peerLast.Load()+timeoutNs < now {
				return nil /* both directions idle: normal timeout */
			}
			continue
		} else if err == io.EOF {
			return nil /* ignore EOF */
		} else if err != nil {
			return err
		}
		selfLast.Store(time.Now().UnixNano())

		if _, err = dst.WriteTo(buf[:n], to); err != nil {
			return err
		}
	}
}

type symmetricNATPacketConn struct {
	net.PacketConn
	src         string
	dst         string
	dstPort     int
	learnedPeer *net.UDPAddr // D1: peer-learning lock — first accepted remote address is pinned
}

func newSymmetricNATPacketConn(pc net.PacketConn, metadata *M.Metadata) *symmetricNATPacketConn {
	return &symmetricNATPacketConn{
		PacketConn: pc,
		src:        metadata.SourceAddress(),
		dst:        metadata.DestinationAddress(),
		dstPort:    int(metadata.DstPort),
	}
}

// ReadFrom 过滤会话外的回包，实施 D1 "对端学习锁定"：
//   - 仍按源端口初筛（NAT64/anycast 首回复兼容）；
//   - 首个被接受的远端地址锁定为会话对端（learnedPeer）；
//   - 后续数据报必须来自锁定对端（IP+端口完全匹配），不匹配则丢弃。
// 这将注入面从"任意主机永久可注入"收窄为"一次性首包竞速"。
func (pc *symmetricNATPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	for {
		n, from, err := pc.PacketConn.ReadFrom(p)
		if err != nil {
			return n, from, err
		}

		if from != nil {
			if ua, ok := from.(*net.UDPAddr); ok {
				if ua.Port != pc.dstPort {
					log.Warnf("[UDP] symmetric NAT %s->%s: drop packet from %s (port mismatch)", pc.src, pc.dst, from)
					continue
				}
				// D1: peer-learning lock
				if pc.learnedPeer == nil {
					pc.learnedPeer = ua
				} else if !pc.learnedPeer.IP.Equal(ua.IP) || pc.learnedPeer.Port != ua.Port {
					log.Warnf("[UDP] symmetric NAT %s->%s: drop packet from %s (peer lock mismatch, expected %s)", pc.src, pc.dst, ua, pc.learnedPeer)
					continue
				}
			} else if from.String() != pc.dst {
				log.Warnf("[UDP] symmetric NAT %s->%s: drop packet from %s", pc.src, pc.dst, from)
				continue
			}
		}

		return n, from, err
	}
}
