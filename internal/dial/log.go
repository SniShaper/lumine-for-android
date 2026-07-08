package dial

import (
	"github.com/lzpls/enimul/internal/log"
)

var logger log.Logger

func SetLogger(l log.Logger) { logger = l }
