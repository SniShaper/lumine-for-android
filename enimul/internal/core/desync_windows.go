//go:build windows

package core

import (
	"io"
	"os"
	"syscall"
	"time"

	E "github.com/lzpls/enimul/internal/errors"

	"golang.org/x/sys/windows"
)

var transmitFileSem chan struct{}

func init() {
	const VER_NT_WORKSTATION = 0x0000001
	if windows.RtlGetVersion().ProductType == VER_NT_WORKSTATION {
		transmitFileSem = make(chan struct{}, 2)
	}
}

func sendWithNoise(
	rawConn syscall.RawConn,
	fakeData, realData []byte,
	fakeTTL, defaultTTL, level, opt int,
	fakeSleep time.Duration,
) error {
	realDataLen := len(realData)
	fakeDataLen := len(fakeData)
	if fakeDataLen != realDataLen {
		return E.NewAny("invalid data length (fake=", fakeDataLen, ",real=", realDataLen, ")")
	}

	var sockHandle windows.Handle
	if err := rawConn.Control(func(fd uintptr) {
		sockHandle = windows.Handle(fd)
	}); err != nil {
		return E.WithStr("raw control", err)
	}

	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		return E.WithStr("create temp file", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return E.WithStr("seek start", err)
	}

	_, err = tmpFile.Write(fakeData)
	if err != nil {
		return E.WithStr("write fake data", err)
	}

	if err = tmpFile.Sync(); err != nil {
		return E.WithStr("sync fake data", err)
	}

	if err = windows.SetsockoptInt(
		sockHandle, level, opt, fakeTTL,
	); err != nil {
		return E.WithStr("set fake ttl", err)
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return E.WithStr("seek start", err)
	}

	he, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return E.WithStr("create event", err)
	}
	defer windows.CloseHandle(he)
	ov := windows.Overlapped{HEvent: he}

	if transmitFileSem != nil {
		transmitFileSem <- struct{}{}
		defer func() { <-transmitFileSem }()
	}

	fileRawConn, err := getRawConn(tmpFile)
	if err != nil {
		return err
	}
	var transmitFileErr error
	rawCtrlErr := fileRawConn.Control(func(fd uintptr) {
		toWrite := uint32(realDataLen)
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
		return E.WithStr("raw control", rawCtrlErr)
	}
	if transmitFileErr != nil && transmitFileErr != windows.ERROR_IO_PENDING {
		return E.WithStr("call TransmitFile", transmitFileErr)
	}

	time.Sleep(fakeSleep)

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return E.WithStr("seek start", err)
	}

	_, err = tmpFile.Write(realData)
	if err != nil {
		return E.WithStr("write real data", err)
	}

	if err = tmpFile.Sync(); err != nil {
		return E.WithStr("sync real data", err)
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return E.WithStr("seek start", err)
	}
	if err = windows.SetsockoptInt(
		sockHandle, level, opt, defaultTTL,
	); err != nil {
		return E.WithStr("set default ttl", err)
	}

	event, err := windows.WaitForSingleObject(ov.HEvent, 5000)
	if err != nil {
		return E.WithStr("wait for TransmitFile", err)
	}

	switch event {
	case windows.WAIT_OBJECT_0:
	case uint32(windows.WAIT_TIMEOUT):
		return E.New("wait for TransmitFile: timeout (5s)")
	case windows.WAIT_ABANDONED:
		return E.New("wait for TransmitFile: WAIT_ABANDONED")
	case windows.WAIT_FAILED:
		return E.WithStr("wait for TransmitFile: WAIT_FAILED", syscall.GetLastError())
	default:
		return E.NewAny("wait for TransmitFile: unexpected event: ", event)
	}

	var n, flags uint32
	if err = windows.WSAGetOverlappedResult(
		sockHandle, &ov, &n, false, &flags,
	); err != nil {
		return E.WithStr("get TransmitFile result", err)
	}
	if int(n) < realDataLen {
		return E.NewAny("sent only ", n, " of ", realDataLen, " bytes")
	}
	return nil
}
