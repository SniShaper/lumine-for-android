package lumine

import (
	"errors"
	"net"

	log "github.com/moi-si/mylog"
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
	return net.JoinHostPort(p.TargetHost, formatInt(p.TargetPort))
}

func (p DialPlan) OriginAddress() string {
	if p.OriginPort <= 0 {
		return p.OriginHost
	}
	return net.JoinHostPort(p.OriginHost, formatInt(p.OriginPort))
}

func PlanRequest(req RequestContext, logger *log.Logger) (DialPlan, error) {
	if req.Host == "" {
		return DialPlan{}, errors.New("request host is empty")
	}

	recoveredDomain := ""
	planningHost := req.Host
	if net.ParseIP(req.Host) != nil {
		recoveredDomain, _ = defaultResolver.LookupDomain(req.Host)
		if recoveredDomain != "" {
			if logger != nil {
				logger.Info("Plan fake-ip recover:", req.Host, "->", recoveredDomain)
			}
			planningHost = recoveredDomain
		} else if logger != nil && isFakeIPAddress(req.Host) {
			logger.Info("Plan fake-ip miss:", req.Host)
		}
	}

	resolveDomain := req.DomainTargetMode != PreserveDomainTarget
	dstHost, policy, failed, blocked, matchedDomain, matchedIP := genPolicyWithOptions(logger, planningHost, resolveDomain)
	if failed {
		return DialPlan{}, errors.New("failed to resolve dial plan")
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
		Policy:          policy,
		Blocked:         blocked,
	}
	if logger != nil && recoveredDomain != "" {
		logger.Info(
			"Plan target:",
			"domain="+recoveredDomain,
			"target="+plan.TargetAddress(),
			"mode="+plan.Policy.Mode.String(),
			"matched_domain="+boolToText(plan.MatchedDomain),
			"matched_ip="+boolToText(plan.MatchedIP),
		)
	}
	return plan, nil
}

func PlanAddress(originHost string, logger *log.Logger) (DialPlan, error) {
	return PlanRequest(RequestContext{Host: originHost}, logger)
}
