package core

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/lzpls/enimul/internal/addrtrie"
	"github.com/lzpls/enimul/internal/dial"
	"github.com/lzpls/enimul/internal/log"

	"github.com/cespare/xxhash/v2"
)

type Config struct {
	LogLevel         log.Level          `json:"log_level"`
	LogOutput        string             `json:"log_output"`
	Socks5Addr       string             `json:"socks5_address"`
	HttpAddr         string             `json:"http_address"`
	OutboundBinding  dial.BindingOption `json:"outbound_binding"`
	DNSConfig        DNSConfig          `json:"dns"`
	TTLProbingConfig TTLProbingConfig   `json:"ttl_probing"`
	FakeTTLRules     string             `json:"fake_ttl_rules"`
	DNSSingleflight  bool               `json:"dns_singleflight"`
	DNSCacheTTL      int                `json:"dns_cache_ttl"`
	DNSCacheCapacity uint32             `json:"dns_cache_cap"`
	TTLSingleflight  bool               `json:"ttl_singleflight"`
	TTLCacheTTL      int                `json:"ttl_cache_ttl"`
	TTLCacheCapacity uint32             `json:"ttl_cache_cap"`
	IPPools          map[string]*IPPool `json:"ip_pools"`
	Hosts            map[string]string  `json:"hosts"`
	DefaultPolicy    Policy             `json:"default_policy"`
	DomainPolicies   map[string]Policy  `json:"domain_policies"`
	IpPolicies       map[string]Policy  `json:"ip_policies"`
}

func LoadConfig(filePath string) (string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	var conf Config
	if err = json.NewDecoder(file).Decode(&conf); err != nil {
		file.Close()
		return "", "", err
	}
	file.Close()

	if err := setLogOutput(conf.LogOutput); err != nil {
		return "", "", err
	}
	logLevel = conf.LogLevel

	if err = dial.SetLocalAddr(conf.OutboundBinding); err != nil {
		return "", "", err
	}
	dial.SetLogger(newLogger("[dial]"))

	if len(conf.IPPools) > 0 {
		ipPools = conf.IPPools
		for tag, pool := range ipPools {
			pool.Init(newLogger("P[" + tag + "]"))
		}
	}

	defaultPolicy = conf.DefaultPolicy

	hostsMatcher = addrtrie.NewDomainMatcher[string]()
	for patterns, host := range conf.Hosts {
		for elem := range strings.SplitSeq(patterns, ";") {
			for _, pattern := range expandPattern(elem) {
				hostsMatcher.Add(pattern, host)
			}
		}
	}

	domainMatcher = addrtrie.NewDomainMatcher[*Policy]()
	for patterns, policy := range conf.DomainPolicies {
		for elem := range strings.SplitSeq(patterns, ";") {
			for _, pattern := range expandPattern(elem) {
				domainMatcher.Add(pattern, &policy)
			}
		}
	}

	ipMatcher = addrtrie.NewIPv4Trie[*Policy]()
	ipv6Matcher = addrtrie.NewIPv6Trie[*Policy]()
	for patterns, policy := range conf.IpPolicies {
		for elem := range strings.SplitSeq(patterns, ";") {
			for _, ipOrNet := range expandPattern(elem) {
				if isIPv6(ipOrNet) {
					ipv6Matcher.Insert(ipOrNet, &policy)
				} else {
					ipMatcher.Insert(ipOrNet, &policy)
				}
			}
		}
	}

	if err = setDNS(conf.DNSConfig); err != nil {
		return "", "", err
	}

	if err = setTTLProbing(conf.TTLProbingConfig); err != nil {
		return "", "", err
	}

	return conf.Socks5Addr, conf.HttpAddr, nil
}

// for freelru
func hashStringXXHASH(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func expandPattern(s string) []string {
	left := -1
	for i := range s {
		if s[i] == '(' {
			left = i
			break
		}
	}

	if left == -1 {
		return splitByPipe(s)
	}

	right := -1
	depth := 1
	for i := left + 1; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				right = i
				break
			}
		}
	}

	if right == -1 {
		return splitByPipe(s)
	}

	prefix := s[:left]
	inner := s[left+1 : right]
	suffix := s[right+1:]

	parts := splitByPipe(inner)

	suffixResults := expandPattern(suffix)

	result := make([]string, 0, len(parts)*len(suffixResults))
	for _, part := range parts {
		for _, suff := range suffixResults {
			result = append(result, prefix+part+suff)
		}
	}

	return result
}

func splitByPipe(s string) []string {
	if s == "" {
		return []string{""}
	}
	result := []string{}
	curr := ""
	for i := range s {
		if s[i] == '|' {
			result = append(result, curr)
			curr = ""
		} else {
			curr += string(s[i])
		}
	}
	result = append(result, curr)
	return result
}
