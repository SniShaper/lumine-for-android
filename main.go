package main

import (
	"flag"
	"os"
	"runtime"
	"sync"

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
	socks5Addr := flag.String("b", "", "SOCKS5 bind address (override config)")
	httpAddr := flag.String("hb", "", "HTTP bind address (override config)")
	sniAddr := flag.String("spb", "", "SNI proxy bind address (override config)")
	maxprocs := flag.Int("mp", 0, "GOMAXPROCS")
	printLicense := flag.Bool("license", false, "Show license and source code information and exit")
	disallowUnknownFields := flag.Bool("duf", false, "Reject config containing unknown fields")
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
	configSocks5Addr, configHTTPAddr, configSNIAddr, err := core.LoadConfig(configPath, *disallowUnknownFields)
	if err != nil {
		F.Println("Failed to load config:", err)
		return
	}

	if *maxprocs > 0 {
		runtime.GOMAXPROCS(*maxprocs)
	}

	startPprofServer()

	var wg sync.WaitGroup
	wg.Go(func() { core.SOCKS5Serve(*socks5Addr, configSocks5Addr) })
	wg.Go(func() { core.HTTPServe(*httpAddr, configHTTPAddr) })
	wg.Go(func() { core.SNIServe(*sniAddr, configSNIAddr) })
	wg.Wait()
}

func showLicense() {
	F.Println("This project is licensed under the GNU Affero General Public License v3.0.")
	F.Println("Source code: https://github.com/lzpls/enimul")
	F.Println("More: https://www.gnu.org/licenses/agpl-3.0.html")
}
