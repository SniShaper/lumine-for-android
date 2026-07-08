package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/lzpls/enimul/internal/core"
	F "github.com/lzpls/enimul/internal/fmt"
)

func main() {
	F.Println("lzpls/enimul", core.Version)
	F.Println()
	flag.Usage = func() {
		flag.PrintDefaults()
		F.Println()
		showLicense()
	}
	confPath := flag.String("c", "", "Config file path (override environment variable ENIMUL_CONFIG_FILE)")
	addr := flag.String("b", "", "SOCKS5 bind address (override config)")
	hAddr := flag.String("hb", "", "HTTP bind address (override config)")
	maxprocs := flag.Int("mp", 0, "GOMAXPROCS")
	printLicense := flag.Bool("license", false, "Show license and source code information and exit")
	flag.Parse()

	if *printLicense {
		showLicense()
		return
	}

	configPath := *confPath
	if configPath == "" {
		configPath = os.Getenv("ENIMUL_CONFIG_FILE")
		if configPath == "" {
			configPath = "config.json"
		}
	}
	socks5Addr, httpAddr, err := core.LoadConfig(configPath)
	if err != nil {
		F.Println("Failed to load config:", err)
		return
	}

	if *maxprocs > 0 {
		runtime.GOMAXPROCS(*maxprocs)
	}

	startPprofServer()

	done := make(chan struct{})
	go core.SOCKS5Accept(addr, socks5Addr, done)
	core.HTTPAccept(hAddr, httpAddr)
	<-done
}

func showLicense() {
	F.Println("This project is licensed under the GNU Affero General Public License v3.0.")
	F.Println("Source code: https://github.com/lzpls/enimul")
	F.Println("More: https://www.gnu.org/licenses/agpl-3.0.html")
}
