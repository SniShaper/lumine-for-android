//go:build linux

package lumine

import (
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const minInterval = 100 * time.Millisecond

func detectMinimalReachableTTL(
	addr string, ipv6 bool,
	maxTTL, attempts int,
	dialTimeout time.Duration,
) (int, error) {
	var level, opt int
	if ipv6 {
		level, opt = unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS
	} else {
		level, opt = unix.IPPROTO_IP, unix.IP_TTL
	}
	dialer := net.Dialer{Timeout: dialTimeout}

	low, high := 1, maxTTL
	found := -1

	for low <= high {
		mid := (low + high) / 2
		dialer.Control = func(_, _ string, c syscall.RawConn) error {
			var innerErr error
			if err := c.Control(func(fd uintptr) {
				innerErr = unix.SetsockoptInt(int(fd), level, opt, mid)
			}); err != nil {
				return wrap("raw control", err)
			}
			if innerErr != nil {
				return wrap("setsockopt", innerErr)
			}
			return nil
		}
		var ok bool
		for range attempts {
			conn, err := dialer.Dial("tcp", addr)
			if err == nil {
				conn.Close()
				ok = true
				break
			}
			if netErr := err.(*net.OpError); !netErr.Timeout() {
				return -1, wrap("dial "+formatInt(mid), err)
			}
		}
		if ok {
			found = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	if ttlCache != nil && found != -1 {
		ttlCache.AddWithLifetime(addr, found, ttlCacheTTL)
	}
	return found, nil
}

func sendWithNoise(
	socketFD int, rawConn syscall.RawConn,
	fakeData, realData []byte,
	fakeTTL, defaultTTL, level, opt int,
	fakeSleep time.Duration,
) error {
	var pipeFDs [2]int
	if err := unix.Pipe2(pipeFDs[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return wrap("create pipe", err)
	}
	pipeR, pipeW := pipeFDs[0], pipeFDs[1]
	defer unix.Close(pipeR)
	defer unix.Close(pipeW)

	pageSize := syscall.Getpagesize()
	nPages := (len(fakeData) + pageSize - 1) / pageSize
	mmapLen := nPages * pageSize
	data, err := unix.Mmap(-1, 0, mmapLen,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_PRIVATE|unix.MAP_ANONYMOUS)
	if err != nil {
		return wrap("mmap", err)
	}
	defer unix.Munmap(data)
	copy(data, fakeData)

	err = unix.SetsockoptInt(socketFD, level, opt, fakeTTL)
	if err != nil {
		return wrap("set fake TTL", err)
	}
	iov := unix.Iovec{
		Base: &data[0],
		Len:  toUint(len(fakeData)),
	}
	if _, err := unix.Vmsplice(pipeW, []unix.Iovec{iov}, unix.SPLICE_F_GIFT); err != nil {
		return wrap("vmsplice", err)
	}

	var rawWriteErr, innerErr error
	done := make(chan struct{})
	go func() {
		toWrite := len(fakeData)
		rawWriteErr = rawConn.Write(func(fd uintptr) (done bool) {
			for toWrite > 0 {
				n, spliceErr := unix.Splice(pipeR, nil, int(fd), nil, toWrite, unix.SPLICE_F_NONBLOCK)
				innerErr = spliceErr
				if innerErr != nil {
					if innerErr == unix.EINTR {
						continue
					}
					return innerErr != unix.EAGAIN
				}
				toWrite -= int(n)
			}
			return true
		})
		close(done)
	}()

	time.Sleep(fakeSleep)

	copy(data, realData) // will be sent automatically by the system.

	err = unix.SetsockoptInt(socketFD, level, opt, defaultTTL)
	if err != nil {
		return wrap("set default TTL", err)
	}
	<-done
	if rawWriteErr != nil {
		return wrap("raw write (splice)", rawWriteErr)
	}
	if innerErr != nil {
		return wrap("splice", innerErr)
	}
	return nil
}

func desyncSend(
	conn net.Conn, ipv6 bool,
	firstPacket []byte, sniPos, sniLen, fakeTTL int, fakeSleep time.Duration,
) error {
	rawConn, err := getTCPRawConn(conn)
	if err != nil {
		return err
	}

	var fd int
	if err = rawConn.Control(func(fileDesc uintptr) {
		fd = int(fileDesc)
	}); err != nil {
		return wrap("raw control", err)
	}

	var level, opt, defaultTTL int
	if ipv6 {
		level = unix.IPPROTO_IPV6
		opt = unix.IPV6_UNICAST_HOPS
	} else {
		level = unix.IPPROTO_IP
		opt = unix.IP_TTL
	}
	defaultTTL, err = unix.GetsockoptInt(fd, level, opt)
	if err != nil {
		return wrap("get default TTL", err)
	}

	if fakeSleep < minInterval {
		fakeSleep = minInterval
	}

	cut, found := findLastDot(firstPacket, sniPos, sniLen)
	var fakeData []byte
	if found {
		fakeData = make([]byte, cut)
		copy(fakeData, firstPacket[:sniPos])
	} else {
		fakeData = firstPacket[:cut]
	}

	if err = sendWithNoise(
		fd, rawConn,
		fakeData,
		firstPacket[:cut],
		fakeTTL,
		defaultTTL,
		level, opt,
		fakeSleep,
	); err != nil {
		return wrap("send data with noise", err)
	}
	if _, err = conn.Write(firstPacket[cut:]); err != nil {
		return wrap("send remaining data", err)
	}
	return nil
}
