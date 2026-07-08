package core

import (
	"io"
	"os"

	E "github.com/lzpls/enimul/internal/errors"
	"github.com/lzpls/enimul/internal/log"
)

var (
	logLevel  log.Level
	logOutput io.Writer
)

func setLogOutput(out string) error {
	switch out {
	case "stderr":
		logOutput = os.Stderr
	case "", "stdout": // default
		logOutput = os.Stdout
	default:
		f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return E.WithStr("open log file", err)
		}
		logOutput = f
	}
	return nil
}

func newLogger(prefix string) log.Logger {
	return log.New(logOutput, prefix, logLevel)
}
