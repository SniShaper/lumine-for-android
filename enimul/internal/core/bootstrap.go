package core

import (
	"context"
	"net"
	"time"

	E "github.com/lzpls/enimul/internal/errors"
)

// bootstrapDNSTimeout and bootstrapCacheTTL are fixed rather than
// config-driven: bootstrap resolution is a one-off fallback for hostnames
// (e.g. a DoH server's own host) that can't rely on the OS resolver being
// reachable, notably inside Android's VPN tun process.
const (
	bootstrapDNSTimeout = 10 * time.Second
	bootstrapCacheTTL   = 5 * time.Minute
)

var bootstrapLookupIP = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func resolveBootstrapHost(host string) (string, error) {
	if host == "" || net.ParseIP(host) != nil {
		return host, nil
	}

	if dnsCache != nil {
		if ip, ok := dnsCache.Get(host); ok {
			return ip, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapDNSTimeout)
	defer cancel()

	addrs, err := bootstrapLookupIP(ctx, host)
	if err != nil {
		return "", E.WithStr("bootstrap resolve "+host, err)
	}
	ip := pickBootstrapIP(addrs)
	if ip == "" {
		return "", E.New("bootstrap resolve " + host + ": no address found")
	}

	if dnsCache != nil {
		dnsCache.AddWithLifetime(host, ip, bootstrapCacheTTL)
	}
	return ip, nil
}

func pickBootstrapIP(addrs []net.IPAddr) string {
	var first string
	for _, addr := range addrs {
		ip := addr.IP
		if ip == nil {
			continue
		}
		if first == "" {
			first = ip.String()
		}
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	return first
}
