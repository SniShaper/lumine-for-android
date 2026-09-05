package com.moi.lumine.model

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class LumineConfig(
    @Json(name = "log_level") val logLevel: String = "INFO",
    @Json(name = "log_output") val logOutput: String? = null,
    @Json(name = "socks5_address") val socks5Address: String = "127.0.0.1:1080",
    @Json(name = "http_address") val httpAddress: String = "127.0.0.1:1225",
    @Json(name = "outbound_binding") val outboundBinding: OutboundBinding? = null,
    @Json(name = "dns") val dns: DnsConfig = DnsConfig(),
    @Json(name = "ttl_probing") val ttlProbing: TtlProbingConfig = TtlProbingConfig(),
    @Json(name = "fake_ttl_rules") val fakeTtlRules: String? = null,
    @Json(name = "dns_singleflight") val dnsSingleflight: Boolean? = null,
    @Json(name = "dns_cache_ttl") val dnsCacheTtl: Int? = null,
    @Json(name = "dns_cache_cap") val dnsCacheCap: Long? = null,
    @Json(name = "ttl_singleflight") val ttlSingleflight: Boolean? = null,
    @Json(name = "ttl_cache_ttl") val ttlCacheTtl: Int? = null,
    @Json(name = "ttl_cache_cap") val ttlCacheCap: Long? = null,
    @Json(name = "ip_pools") val ipPools: Map<String, IPPool>? = null,
    @Json(name = "hosts") val hosts: Map<String, String>? = null,
    @Json(name = "default_policy") val defaultPolicy: Policy = Policy(),
    @Json(name = "ip_policies") val ipPolicies: Map<String, Policy> = emptyMap(),
    @Json(name = "domain_policies") val domainPolicies: Map<String, Policy> = emptyMap()
)

@JsonClass(generateAdapter = true)
data class OutboundBinding(
    @Json(name = "enable_for_android") val enableForAndroid: Boolean? = null,
    @Json(name = "method") val method: String? = null,
    @Json(name = "manual_select") val manualSelect: Boolean? = null,
    @Json(name = "zone") val zone: String? = null,
    @Json(name = "preferred_prefix") val preferredPrefix: String? = null,
    @Json(name = "update_interval") val updateInterval: String? = null,
    @Json(name = "dial_tcp") val dialTcp: Boolean? = null,
    @Json(name = "dial_ipv4_target") val dialIpv4Target: String? = null,
    @Json(name = "dial_ipv6_target") val dialIpv6Target: String? = null,
    @Json(name = "dial_timeout") val dialTimeout: String? = null,
    @Json(name = "custom_ipv4") val customIpv4: String? = null,
    @Json(name = "custom_ipv6") val customIpv6: String? = null,
    @Json(name = "custom_zone") val customZone: String? = null
)

@JsonClass(generateAdapter = true)
data class IPPool(
    @Json(name = "ips") val ips: List<String>? = null,
    @Json(name = "port") val port: Int? = null,
    @Json(name = "wait_scan_on_startup") val waitScanOnStartup: Boolean? = null,
    @Json(name = "fallback_ip") val fallbackIp: String? = null,
    @Json(name = "top_ip_count") val topIpCount: Int? = null,
    @Json(name = "max_concurrency") val maxConcurrency: Int? = null,
    @Json(name = "timeout") val timeout: String? = null,
    @Json(name = "update_interval") val updateInterval: String? = null,
    @Json(name = "attempts") val attempts: Int? = null
)

@JsonClass(generateAdapter = true)
data class DnsConfig(
    @Json(name = "type") val type: String = "https",
    @Json(name = "addr") val addr: String = "https://xwfpeb16ii.cloudflare-gateway.com/dns-query",
    @Json(name = "resolvers") val resolvers: List<String>? = null,
    @Json(name = "client_timeout") val clientTimeout: String = "5s",
    @Json(name = "wait_timeout") val waitTimeout: String = "400ms",
    @Json(name = "min_rtt") val minRtt: String = "",
    @Json(name = "udp_size") val udpSize: Int = 4096,
    @Json(name = "singleflight") val singleFlight: Boolean = false,
    @Json(name = "disable_cache") val disableCache: Boolean = false,
    @Json(name = "cache_capacity") val cacheCapacity: Int = 4096,
    @Json(name = "edns0_subnet") val edns0Subnet: String = "",
    @Json(name = "doh_socks5_addr") val dohSocks5Addr: String = ""
)

@JsonClass(generateAdapter = true)
data class TtlProbingConfig(
    @Json(name = "fake_ttl_rules") val fakeTtlRules: String = "0-1;3=3;5-1;8-2;13-3;20=18",
    @Json(name = "singleflight") val singleFlight: Boolean = false,
    @Json(name = "disable_cache") val disableCache: Boolean = false,
    @Json(name = "cache_capacity") val cacheCapacity: Int = 1024
)

@JsonClass(generateAdapter = true)
data class Policy(
    @Json(name = "reply_first") val replyFirst: Boolean? = null,
    @Json(name = "sniff_override") val sniffOverride: String? = null,
    @Json(name = "dns_mode") val dnsMode: String? = null,
    @Json(name = "dns_cache_ttl") val dnsCacheTtl: String? = null,
    @Json(name = "connect_timeout") val connectTimeout: String? = null,
    @Json(name = "tls13_only") val tls13Only: Boolean? = null,
    @Json(name = "http_status") val httpStatus: Int? = null,
    @Json(name = "mode") val mode: String? = null,
    @Json(name = "minor_ver") val minorVer: Int? = null,
    @Json(name = "num_records") val numRecords: Int? = null,
    @Json(name = "num_segs") val numSegs: Int? = null,
    @Json(name = "wait_for_ack") val waitForAck: Boolean? = null,
    @Json(name = "send_interval") val sendInterval: String? = null,
    @Json(name = "oob") val oob: Boolean? = null,
    @Json(name = "oob_ex") val oobEx: Boolean? = null,
    @Json(name = "fake_ttl") val fakeTtl: Int? = null,
    @Json(name = "fake_sleep") val fakeSleep: String? = null,
    @Json(name = "attempts") val attempts: Int? = null,
    @Json(name = "max_ttl") val maxTtl: Int? = null,
    @Json(name = "single_timeout") val singleTimeout: String? = null,
    @Json(name = "ttl_cache_ttl") val ttlCacheTtl: String? = null,
    @Json(name = "host") val host: String? = null,
    @Json(name = "map_to") val mapTo: String? = null,
    @Json(name = "nat64_prefix") val nat64Prefix: String? = null
)
