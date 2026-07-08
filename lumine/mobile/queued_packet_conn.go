package mobile

import (
	"net"
	"sync"
	"time"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type queuedPacket struct {
	payload []byte
	addr    net.Addr
}

type queuedPacketConn struct {
	base net.PacketConn

	packets chan queuedPacket
	closed  chan struct{}

	deadlineMu   sync.RWMutex
	readDeadline time.Time
}

func newQueuedPacketConn(base net.PacketConn) *queuedPacketConn {
	pc := &queuedPacketConn{
		base:    base,
		packets: make(chan queuedPacket, 32),
		closed:  make(chan struct{}),
	}
	go pc.pumpBaseReads()
	return pc
}

func (pc *queuedPacketConn) pumpBaseReads() {
	for {
		buf := make([]byte, 64*1024)
		n, addr, err := pc.base.ReadFrom(buf)
		if err != nil {
			select {
			case <-pc.closed:
				return
			default:
				return
			}
		}

		packet := queuedPacket{
			payload: append([]byte(nil), buf[:n]...),
			addr:    addr,
		}

		select {
		case pc.packets <- packet:
		case <-pc.closed:
			return
		}
	}
}

func (pc *queuedPacketConn) EnqueuePacket(payload []byte, addr net.Addr) error {
	packet := queuedPacket{
		payload: append([]byte(nil), payload...),
		addr:    addr,
	}

	select {
	case pc.packets <- packet:
		return nil
	case <-pc.closed:
		return net.ErrClosed
	}
}

func (pc *queuedPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	var deadline <-chan time.Time
	if t := pc.getReadDeadline(); !t.IsZero() {
		wait := time.Until(t)
		if wait <= 0 {
			return 0, nil, timeoutError{}
		}
		timer := time.NewTimer(wait)
		defer timer.Stop()
		deadline = timer.C
	}

	select {
	case packet := <-pc.packets:
		n := copy(p, packet.payload)
		return n, packet.addr, nil
	case <-deadline:
		return 0, nil, timeoutError{}
	case <-pc.closed:
		return 0, nil, net.ErrClosed
	}
}

func (pc *queuedPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	return pc.base.WriteTo(p, addr)
}

func (pc *queuedPacketConn) Close() error {
	select {
	case <-pc.closed:
	default:
		close(pc.closed)
	}
	return pc.base.Close()
}

func (pc *queuedPacketConn) LocalAddr() net.Addr {
	return pc.base.LocalAddr()
}

func (pc *queuedPacketConn) SetDeadline(t time.Time) error {
	if err := pc.SetReadDeadline(t); err != nil {
		return err
	}
	return pc.base.SetWriteDeadline(t)
}

func (pc *queuedPacketConn) SetReadDeadline(t time.Time) error {
	pc.deadlineMu.Lock()
	pc.readDeadline = t
	pc.deadlineMu.Unlock()
	return nil
}

func (pc *queuedPacketConn) SetWriteDeadline(t time.Time) error {
	return pc.base.SetWriteDeadline(t)
}

func (pc *queuedPacketConn) getReadDeadline() time.Time {
	pc.deadlineMu.RLock()
	defer pc.deadlineMu.RUnlock()
	return pc.readDeadline
}
