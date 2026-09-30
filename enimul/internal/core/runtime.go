package core

import E "github.com/lzpls/enimul/internal/errors"

func ResetRuntimeState() error {
	runtimeStateMu.RLock()
	dnsConf, ttlConf := lastDNSConfig, lastTTLProbingConfig
	runtimeStateMu.RUnlock()
	if err := setDNS(dnsConf); err != nil {
		return E.WithStr("reset dns runtime state", err)
	}
	if err := setTTLProbing(ttlConf); err != nil {
		return E.WithStr("reset ttl probing runtime state", err)
	}
	return nil
}
