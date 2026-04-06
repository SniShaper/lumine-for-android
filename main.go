package main

import (
	"flag"
	"fmt"

	lumine "github.com/moi-si/lumine/internal"
)

func main() {
	fmt.Println("moi-si/lumine v0.7.9")
	fmt.Println("")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	configPath := flag.String("c", "config.json", "Config file path")
	addr := flag.String("b", "", "SOCKS5 bind address (default: address from config file)")
	hAddr := flag.String("hb", "", "HTTP bind address (default: address from config file)")
	flag.Parse()

	socks5Addr, httpAddr, err := lumine.LoadConfig(*configPath)
	if err != nil {
		fmt.Println("Failed to load config:", err)
		return
	}

	done := make(chan struct{})
	go lumine.SOCKS5Accept(addr, socks5Addr, done)
	go lumine.HTTPAccept(hAddr, httpAddr, done)
	select {}
}
