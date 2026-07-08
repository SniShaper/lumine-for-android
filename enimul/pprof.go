//go:build !nodebug

package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"

	"github.com/lzpls/enimul/internal/log"
)

var pprofServerAddr = flag.String("dp", "", "Debug pprof server bind address")

func startPprofServer() {
	if *pprofServerAddr == "" {
		return
	}
	go func() {
		if err := http.ListenAndServe(*pprofServerAddr, nil); err != nil {
			log.Err("Debug pprof server listen and serve: ", err)
		}
	}()
}
