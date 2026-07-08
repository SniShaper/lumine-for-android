package lumine

import (
	_ "embed"
	"encoding/base64"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/moi-si/addrtrie"
)

//go:embed gfwlist.txt
var embeddedGFWList string

var (
	gfwDomainMatcher *addrtrie.DomainMatcher[struct{}]
	gfwBypassMatcher *addrtrie.DomainMatcher[struct{}]

	gfwHostRegexp = regexp.MustCompile(`([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)`)
)

func loadBuiltinGFWList() error {
	blocked, bypass, err := parseEmbeddedGFWList(embeddedGFWList)
	if err != nil {
		return err
	}

	gfwDomainMatcher = addrtrie.NewDomainMatcher[struct{}]()
	for _, pattern := range blocked {
		if err := gfwDomainMatcher.Add(pattern, struct{}{}); err != nil {
			return err
		}
	}

	gfwBypassMatcher = addrtrie.NewDomainMatcher[struct{}]()
	for _, pattern := range bypass {
		if err := gfwBypassMatcher.Add(pattern, struct{}{}); err != nil {
			return err
		}
	}

	return nil
}

func parseEmbeddedGFWList(raw string) (blocked []string, bypass []string, err error) {
	payload := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, raw)

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, nil, wrap("decode embedded gfwlist", err)
	}

	blockedSet := make(map[string]struct{})
	bypassSet := make(map[string]struct{})
	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "[") {
			continue
		}

		targetSet := blockedSet
		if strings.HasPrefix(line, "@@") {
			targetSet = bypassSet
			line = strings.TrimPrefix(line, "@@")
		}

		pattern, ok := normalizeGFWListRule(line)
		if !ok {
			continue
		}
		targetSet[pattern] = struct{}{}
	}

	blocked = mapKeysSorted(blockedSet)
	bypass = mapKeysSorted(bypassSet)
	return blocked, bypass, nil
}

func normalizeGFWListRule(rule string) (string, bool) {
	rule = strings.TrimSpace(rule)
	if rule == "" || strings.HasPrefix(rule, "/") || strings.HasPrefix(rule, "||^") {
		return "", false
	}

	host, ok := extractGFWListHost(rule)
	if !ok {
		return "", false
	}

	return "*" + host, true
}

func extractGFWListHost(rule string) (string, bool) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return "", false
	}

	if idx := strings.IndexAny(rule, "^$"); idx >= 0 {
		rule = rule[:idx]
	}

	rule = strings.TrimLeft(rule, "|.")
	if rule == "" {
		return "", false
	}

	if strings.Contains(rule, "://") {
		u, err := url.Parse(rule)
		if err == nil && u.Host != "" {
			rule = u.Host
		}
	}

	rule = strings.Trim(rule, "*./")
	if host, _, err := net.SplitHostPort(rule); err == nil {
		rule = host
	}

	match := gfwHostRegexp.FindString(strings.ToLower(rule))
	if match == "" || net.ParseIP(match) != nil {
		return "", false
	}
	return strings.Trim(match, "."), true
}

func mapKeysSorted(m map[string]struct{}) []string {
	items := make([]string, 0, len(m))
	for key := range m {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func isGFWDomain(domain string) bool {
	if domain == "" || gfwDomainMatcher == nil {
		return false
	}
	domain = strings.ToLower(domain)
	if gfwBypassMatcher != nil {
		if _, bypass := gfwBypassMatcher.Find(domain); bypass {
			return false
		}
	}
	_, matched := gfwDomainMatcher.Find(domain)
	return matched
}
