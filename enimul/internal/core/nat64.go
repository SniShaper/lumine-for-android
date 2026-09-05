package core

import (
	"fmt"
	"net"
	"strings"

	E "github.com/lzpls/enimul/internal/errors"
	"github.com/lzpls/enimul/internal/log"
)

// mapNAT64Addr 将 IPv4 地址映射到 NAT64 前缀下的 IPv6 地址，
// 语义与桌面版 SniShaper 保持一致：
//   - prefix 为空：不映射，原样返回 (ok=true)
//   - ip 无法解析：不映射，原样返回 (ok=true)
//   - ip 是原生 IPv6：不支持映射，返回 (ok=false)，由调用方决定丢弃
//   - prefix 为 "2001:67c:2960:6464::" 或 "2001:67c:2960:6464::/96"：
//     取前缀地址前 12 字节 + IPv4 的 4 字节，拼成 16 字节 IPv6
//
// 映射结果例如 151.101.195.42 -> 2001:67c:2960:6464::9765:c32a。
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

// planNat64 对选定的目标应用 NAT64 映射：
//   - 目标仍是域名时先按策略 DNS 模式解析（结果会写入 DNS 缓存）
//   - IPv4 目标映射为前缀下的 IPv6；原生 IPv6 目标按桌面版语义拒绝
//   - 未启用 NAT64 或目标无法映射（非法前缀/域名解析失败）时保持原目标
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
