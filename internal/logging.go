package lumine

import (
	"io"
	"os"

	log "github.com/moi-si/mylog"
)

var coreLogWriter io.Writer = os.Stdout

func SetLogWriter(w io.Writer) {
	if w == nil {
		coreLogWriter = os.Stdout
		return
	}
	coreLogWriter = w
}

func newLogger(prefix string) *log.Logger {
	return log.New(coreLogWriter, prefix, log.LstdFlags, logLevel)
}
