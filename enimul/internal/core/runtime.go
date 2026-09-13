package core

import E "github.com/lzpls/enimul/internal/errors"

// ResetRuntimeState 在底层默认网络切换后调用：以当前配置重建 DNS/反向解析/
// TTL 缓存（等价于清空后重建），使后续解析走当前网络的 DNS/DoH 路径。
// 与桌面端在网络环境变化后重建拨号环境的行为对齐；不重置会话统计（见 ResetStats）。
// 重建失败时返回错误，由调用方记录/处理；原配置保留在 lastDNSConfig/
// lastTTLProbingConfig 中，后续调用可重试。
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
