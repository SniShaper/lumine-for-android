//go:build windows

package lumine

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const minInterval = 100 * time.Millisecond

func detectMinimalReachableTTL(
	addr string, ipv6 bool,
	maxTTL, attempts int,
	dialTimeout time.Duration,
) (int, error) {
	var level, opt int
	if ipv6 {
		level, opt = windows.IPPROTO_IPV6, windows.IPV6_UNICAST_HOPS
	} else {
		level, opt = windows.IPPROTO_IP, windows.IP_TTL
	}
	dialer := net.Dialer{Timeout: dialTimeout}

	low, high := 1, maxTTL
	found := -1

	for low <= high {
		mid := (low + high) / 2
		dialer.Control = func(_, _ string, c syscall.RawConn) error {
			var innerErr error
			if err := c.Control(func(fd uintptr) {
				innerErr = windows.SetsockoptInt(windows.Handle(fd), level, opt, mid)
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
	sockHandle windows.Handle,
	fakeData, realData []byte,
	fakeTTL, defaultTTL, level, opt int,
	fakeSleep time.Duration,
) error {
	if len(fakeData) != len(realData) {
		return errors.New("the length of realData must equal to that of fakeData")
	}

	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		return wrap("create temp file", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return wrap("seek start", err)
	}

	_, err = tmpFile.Write(fakeData)
	if err != nil {
		return wrap("write fake data", err)
	}

	if err = tmpFile.Sync(); err != nil {
		return wrap("sync fake data", err)
	}

	if err = windows.SetsockoptInt(sockHandle, level, opt, fakeTTL); err != nil {
		return wrap("set fake TTL", err)
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return wrap("seek start", err)
	}

	var ov windows.Overlapped
	ov.HEvent, err = windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return wrap("create event", err)
	}
	defer windows.CloseHandle(ov.HEvent)

	if sem != nil {
		sem <- struct{}{}
		defer func() { <-sem }()
	}

	rawConn, err := tmpFile.SyscallConn()
	if err != nil {
		return wrap("get raw conn", err)
	}
	var transmitFileErr error
	rawCtrlErr := rawConn.Control(func(fd uintptr) {
		toWrite := uint32(len(fakeData))
		transmitFileErr = windows.TransmitFile(
			sockHandle,
			windows.Handle(fd),
			toWrite,
			toWrite,
			&ov,
			nil,
			windows.TF_USE_KERNEL_APC|windows.TF_WRITE_BEHIND,
		)
	})
	if rawCtrlErr != nil {
		return wrap("raw control", rawCtrlErr)
	}
	if transmitFileErr != nil && transmitFileErr != windows.ERROR_IO_PENDING {
		return wrap("call TransmitFile", err)
	}

	time.Sleep(fakeSleep)

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return wrap("seek start", err)
	}

	_, err = tmpFile.Write(realData)
	if err != nil {
		return wrap("write real data", err)
	}

	if err = tmpFile.Sync(); err != nil {
		return wrap("sync real data", err)
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return wrap("seek start", err)
	}
	if err = windows.SetsockoptInt(sockHandle, level, opt, defaultTTL); err != nil {
		return wrap("set default TTL", err)
	}

	event, err := windows.WaitForSingleObject(ov.HEvent, 5000)
	if err != nil {
		return wrap("wait for event", err)
	}

	switch event {
	case windows.WAIT_OBJECT_0:
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		return errors.New("TransmitFile: timeout (5s)")
	case windows.WAIT_ABANDONED:
		return errors.New("TransmitFile: WAIT_ABANDONED")
	case windows.WAIT_FAILED:
		return wrap("TransmitFile: WAIT_FAILED", windows.GetLastError())
	default:
		return errors.New("TransmitFile: unexpected event: " + formatUint(event))
	}
}

func desyncSend(
	conn net.Conn, ipv6 bool,
	firstPacket []byte, sniPos, sniLen, fakeTTL int, fakeSleep time.Duration,
) error {
	rawConn, err := getTCPRawConn(conn)
	if err != nil {
		return err
	}

	var sockHandle windows.Handle
	controlErr := rawConn.Control(func(fd uintptr) {
		sockHandle = windows.Handle(fd)
	})
	if controlErr != nil {
		return wrap("raw control", err)
	}

	var level, opt int
	if ipv6 {
		level = windows.IPPROTO_IPV6
		opt = windows.IPV6_UNICAST_HOPS
	} else {
		level = windows.IPPROTO_IP
		opt = windows.IP_TTL
	}
	defaultTTL, err := windows.GetsockoptInt(sockHandle, level, opt)
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
		sockHandle,
		fakeData, firstPacket[:cut],
		fakeTTL, defaultTTL,
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
