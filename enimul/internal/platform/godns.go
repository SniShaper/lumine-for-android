//go:build godns

package platform

import (
	"context"
	"net"
	"time"
)

const (
	dns1 = "8.8.8.8:53"
	dns2 = "1.1.1.1:53"
)

var dnsDialer = net.Dialer{Timeout: 3 * time.Second}

func init() {
	// For systems that cannot access the system DNS server addresses
	// like Android.
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, err := dnsDialer.DialContext(ctx, network, dns1)
		if err == nil {
			return conn, nil
		}
		// Fall back to the secondary resolver instead of returning an
		// inverted (nil conn, nil err) pair.
		return dnsDialer.DialContext(ctx, network, dns2)
	}
}
