package core

// ResetRuntimeState 在底层默认网络切换后调用：以当前配置重建 DNS/反向解析/
// TTL 缓存（等价于清空后重建），使后续解析走当前网络的 DNS/DoH 路径。
// 与桌面端在网络环境变化后重建拨号环境的行为对齐；不重置会话统计（见 ResetStats）。
func ResetRuntimeState() {
	_ = setDNS(lastDNSConfig)
	_ = setTTLProbing(lastTTLProbingConfig)
}
