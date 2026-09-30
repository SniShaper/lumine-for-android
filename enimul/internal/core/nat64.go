package core

import (
	"fmt"
	"net"
	"strings"

	E "github.com/lzpls/enimul/internal/errors"
	"github.com/lzpls/enimul/internal/log"
)

//
func mapNAT64Addr(ipStr string, prefix string) (string, bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ipStr, true
	}
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return ipStr, true
	}
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return ipStr, false
	}

	var prefixIP net.IP
	if strings.Contains(prefix, "/") {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err == nil && ipnet != nil {
			prefixIP = ipnet.IP
		}
	} else {
		prefixIP = net.ParseIP(prefix)
	}

	if prefixIP == nil || len(prefixIP) != 16 {
		return ipStr, true
	}
	mappedIP := make(net.IP, 16)
	copy(mappedIP, prefixIP[:12])
	copy(mappedIP[12:], ipv4)
	return mappedIP.String(), true
}

func isNAT64Enabled(p *Policy) bool {
	return p != nil && strings.TrimSpace(p.Nat64Prefix) != ""
}

func planNat64(logger log.Logger, host string, p *Policy) (string, error) {
	if !isNAT64Enabled(p) {
		return host, nil
	}

	dstHost := host
	if net.ParseIP(dstHost) == nil {
		ip, cached, err := dnsResolve(dstHost, p.DNSMode, p.DNSCacheTTL)
		if err != nil {
			return "", E.WithStr("nat64 resolve "+host, err)
		}
		dstHost = ip
		if cached {
			logger.Info("DNS (cached): ", host, " -> ", dstHost)
		} else {
			logger.Info("DNS: ", host, " -> ", dstHost)
		}
	}

	mapped, ok := mapNAT64Addr(dstHost, p.Nat64Prefix)
	if !ok {
		return "", fmt.Errorf("native IPv6 %s excluded under NAT64 mode", dstHost)
	}
	if mapped != dstHost {
		logger.Info("NAT64: ", dstHost, " -> ", mapped)
	}
	return mapped, nil
}
