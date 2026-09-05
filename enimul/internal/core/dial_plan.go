package core

import (
	"net"

	E "github.com/lzpls/enimul/internal/errors"
	F "github.com/lzpls/enimul/internal/fmt"
	"github.com/lzpls/enimul/internal/log"
)

type RequestSource string

const (
	RequestSourceUnknown RequestSource = ""
	RequestSourceMobile  RequestSource = "mobile_tun"
	RequestSourceSOCKS5  RequestSource = "socks5"
	RequestSourceHTTP    RequestSource = "http"
)

type DomainTargetMode uint8

const (
	ResolveDomainTarget DomainTargetMode = iota
	PreserveDomainTarget
)

type RequestContext struct {
	Source           RequestSource
	Host             string
	Port             int
	DomainTargetMode DomainTargetMode
}

type DialPlan struct {
	Source          RequestSource
	OriginHost      string
	OriginPort      int
	RecoveredDomain string
	MatchedDomain   bool
	MatchedIP       bool
	TargetHost      string
	TargetPort      int
	Policy          Policy
	Blocked         bool
}

func (p DialPlan) TargetAddress() string {
	if p.TargetPort <= 0 {
		return p.TargetHost
	}
	return net.JoinHostPort(p.TargetHost, F.Int(p.TargetPort))
}

func (p DialPlan) OriginAddress() string {
	if p.OriginPort <= 0 {
		return p.OriginHost
	}
	return net.JoinHostPort(p.OriginHost, F.Int(p.OriginPort))
}

// PlanRequest resolves a routing decision for a single outbound request,
// unifying what the HTTP, SOCKS5, and mobile-tun code paths all need:
// fake-IP domain recovery, explicit domain_policies/ip_policies matching
// (genPolicy already applies these), and a built-in GFW-list fallback for
// domains with no explicit rule (genPolicy has no such fallback concept).
func PlanRequest(req RequestContext, logger log.Logger) (DialPlan, error) {
	if req.Host == "" {
		return DialPlan{}, E.New("request host is empty")
	}

	recoveredDomain := ""
	planningHost := req.Host
	isIP := net.ParseIP(req.Host) != nil
	if isIP {
		recoveredDomain, _ = defaultResolver.LookupDomain(req.Host)
		if recoveredDomain != "" {
			if logger != nil {
				logger.Info("Plan fake-ip recover: ", req.Host, " -> ", recoveredDomain)
			}
			planningHost = recoveredDomain
			isIP = false
		} else if logger != nil && isFakeIPAddress(req.Host) {
			logger.Info("Plan fake-ip miss: ", req.Host)
		}
	}

	// genPolicy doesn't expose whether a domain_policies/ip_policies rule
	// matched, only the final merged Policy - derive it ourselves so
	// DialPlan.MatchedDomain/MatchedIP can drive caller-side log verbosity,
	// and so we know whether the GFW-list fallback below should apply.
	matchedDomain, matchedIP := false, false
	if isIP {
		_, matchedIP = getIPPolicy(planningHost)
	} else {
		_, matchedDomain = domainMatcher.Find(planningHost)
	}

	returnWhenDomainNotFound := req.DomainTargetMode == PreserveDomainTarget
	dstHost, policy, failed, blocked, domainNotFound := genPolicy(logger, planningHost, isIP, returnWhenDomainNotFound)
	if failed {
		return DialPlan{}, E.New("failed to resolve dial plan")
	}
	if blocked {
		return DialPlan{
			Source:          req.Source,
			OriginHost:      req.Host,
			OriginPort:      req.Port,
			RecoveredDomain: recoveredDomain,
			MatchedDomain:   matchedDomain,
			MatchedIP:       matchedIP,
			Blocked:         true,
		}, nil
	}
	if domainNotFound {
		// genPolicy returns a nil Policy and an empty dstHost here (it
		// stopped short of resolving, per PreserveDomainTarget); keep the
		// request's own host as the target and fall back to defaultPolicy.
		dstHost = planningHost
		policy = &defaultPolicy
	}

	if !isIP && !matchedDomain {
		switch classifyDomainRoute(planningHost) {
		case domainRouteGFW:
			policy = mergePolicies(&gfwFallbackPolicy, &defaultPolicy)
			matchedDomain = true
		case domainRoutePlain:
			policy = mergePolicies(&plainFallbackPolicy, &defaultPolicy)
		}
	}

	if !matchedIP && dstHost != "" && net.ParseIP(dstHost) != nil {
		if _, found := getIPPolicy(dstHost); found {
			matchedIP = true
		}
	}

	// NAT64: 将选定的目标（或 host 覆盖后的域名）经策略前缀映射到 IPv6。
	// 分片/直连/desync 等模式均可叠加使用，语义与桌面版 SniShaper 一致。
	// fake-IP 未能还原出域名的请求不参与映射，避免把虚拟地址映射成无意义 IPv6。
	if isNAT64Enabled(policy) && (recoveredDomain != "" || !isFakeIPAddress(dstHost)) {
		mapped, err := planNat64(logger, dstHost, policy)
		if err != nil {
			if logger != nil {
				logger.Error("NAT64 plan:", err)
			}
			return DialPlan{}, err
		}
		dstHost = mapped
	}

	targetPort := req.Port
	if policy.Port > 0 {
		targetPort = policy.Port
	}

	plan := DialPlan{
		Source:          req.Source,
		OriginHost:      req.Host,
		OriginPort:      req.Port,
		RecoveredDomain: recoveredDomain,
		MatchedDomain:   matchedDomain,
		MatchedIP:       matchedIP,
		TargetHost:      dstHost,
		TargetPort:      targetPort,
		Policy:          *policy,
		Blocked:         false,
	}
	if logger != nil && recoveredDomain != "" {
		logger.Info(
			"Plan target: domain=", recoveredDomain,
			" target=", plan.TargetAddress(),
			" mode=", plan.Policy.Mode.String(),
			" matched_domain=", plan.MatchedDomain,
			" matched_ip=", plan.MatchedIP,
		)
	}
	return plan, nil
}

func PlanAddress(originHost string, logger log.Logger) (DialPlan, error) {
	return PlanRequest(RequestContext{Host: originHost}, logger)
}
