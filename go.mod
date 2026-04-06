module github.com/moi-si/lumine

go 1.25.5

require golang.org/x/sys v0.42.0

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/elastic/go-freelru v0.16.0
	github.com/miekg/dns v1.1.72
	github.com/moi-si/addrtrie v0.3.0
	github.com/moi-si/mylog v0.2.0
	github.com/xjasonlyu/tun2socks/v2 v2.6.0
	golang.org/x/net v0.52.0
	golang.org/x/sync v0.20.0
)

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/go-chi/chi/v5 v5.2.5 // indirect
	github.com/go-chi/cors v1.2.2 // indirect
	github.com/go-chi/render v1.0.3 // indirect
	github.com/go-gost/relay v0.5.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/schema v1.4.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa // indirect
	golang.org/x/mobile v0.0.0-20260312152759-81488f6aeb60 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20250521234502-f333402bd9cb // indirect
	gvisor.dev/gvisor v0.0.0-20250523182742-eede7a881b20 // indirect
)

replace github.com/xjasonlyu/tun2socks/v2 => ./tun2socks
