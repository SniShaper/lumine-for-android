package log_test

import (
	"errors"
	"io"
	stdlog "log"
	"testing"

	"github.com/lzpls/enimul/internal/log"
)

var err = errors.New("this is an error")

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

var destination = "example.com:443"

func BenchmarkDisabled(b *testing.B) {
	logger := log.New(discard{}, "", log.Disabled)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Connection to ", destination, " failed: ", err)
		}
	})
}

func BenchmarkStdDisabled(b *testing.B) {
	logger := stdlog.New(io.Discard, "", stdlog.LstdFlags)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Print("Connection to ", destination, " failed: ", err)
		}
	})
}

func BenchmarkInfo(b *testing.B) {
	logger := log.New(discard{}, "", log.LevelTrace)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Connection to ", destination, " failed: ", err)
		}
	})
}

func BenchmarkStdInfo(b *testing.B) {
	logger := stdlog.New(discard{}, "", stdlog.LstdFlags)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Print("Connection to ", destination, " failed: ", err)
		}
	})
}
