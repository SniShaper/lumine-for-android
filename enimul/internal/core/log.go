package core

import (
	"io"
	"os"
	"sync"

	E "github.com/lzpls/enimul/internal/errors"
	"github.com/lzpls/enimul/internal/log"
)

var logLevel log.Level

var (
	logWriterMu      sync.RWMutex
	runtimeLogWriter io.Writer = os.Stdout
)

// swappableWriter lets every logger constructed by newLogger follow the
// current runtimeLogWriter, including loggers created before a later
// SetLogWriter call. Android calls SetLogWriter around StartLumine/
// StopLumine, concurrently with in-flight per-connection loggers being
// constructed, so redirecting only future loggers (the plain
// setLogOutput-once behavior) isn't enough.
type swappableWriter struct{}

func (swappableWriter) Write(p []byte) (int, error) {
	logWriterMu.RLock()
	w := runtimeLogWriter
	logWriterMu.RUnlock()
	return w.Write(p)
}

// SetLogWriter redirects all engine log output to w at runtime. Passing nil
// resets it to os.Stdout.
func SetLogWriter(w io.Writer) {
	logWriterMu.Lock()
	if w == nil {
		runtimeLogWriter = os.Stdout
	} else {
		runtimeLogWriter = w
	}
	logWriterMu.Unlock()
}

func setLogOutput(out string) error {
	var w io.Writer
	switch out {
	case "stderr":
		w = os.Stderr
	case "", "stdout": // default
		w = os.Stdout
	default:
		f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return E.WithStr("open log file", err)
		}
		w = f
	}
	SetLogWriter(w)
	return nil
}

func newLogger(prefix string) log.Logger {
	return log.New(swappableWriter{}, prefix, logLevel)
}
