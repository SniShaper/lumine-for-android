package com.moi.lumine.network

import android.content.Context
import android.net.ConnectivityManager
import android.net.LinkProperties
import android.net.Network
import android.net.NetworkCapabilities
import android.util.Log
import com.moi.lumine.model.LumineConfig
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.Inet6Address
import java.net.InetAddress
import java.net.URL
import java.util.concurrent.atomic.AtomicBoolean

data class NetworkStatus(
    val ipv6Available: Boolean? = null,
    val nat64Prefix: String? = null,
    val hasDefaultNetwork: Boolean = true,
    val lastEvent: String? = null,
    val lastEventAt: Long = 0L
)

/**
 * 底层网络环境监控（对齐桌面端 IPv6 可用性/网络变化处理）：
 * - 监听默认网络变化（Wi-Fi <-> 蜂窝 / onLost / onAvailable）；
 * - 由 LinkProperties 推导 IPv6 可用性（ULA 计入，语义与桌面 net.Interfaces 检查一致）；
 * - 仅 IPv4 网络时按 RFC 7050 探测运营商 NAT64 前缀（ipv4only.arpa AAAA 内嵌 192.0.0.170/171）；
 * - 网络变更后通知 Go 引擎清缓存（Mobile.onNetworkChanged）。
 */
object NetworkMonitor {
    private const val TAG = "NetworkMonitor"
    private const val CHANGE_DEBOUNCE_MS = 500L

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val _status = MutableStateFlow(NetworkStatus())
    val status: StateFlow<NetworkStatus> = _status

    private var manager: ConnectivityManager? = null
    private var callback: ConnectivityManager.NetworkCallback? = null
    private val registered = AtomicBoolean(false)
    private var lastChangeAt = 0L
    private var detectJob: Job? = null

    fun start(context: Context) {
        if (registered.get()) return
        val app = context.applicationContext
        val cm = app.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return
        manager = cm
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                handleEvent(app, cm, network, "网络可用")
            }

            override fun onLost(network: Network) {
                handleEvent(app, cm, null, "网络丢失")
            }

            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                if (capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)) {
                    handleEvent(app, cm, network, "网络已切换")
                }
            }
        }
        if (!registered.compareAndSet(false, true)) return
        callback = cb
        runCatching { cm.registerDefaultNetworkCallback(cb) }
            .onSuccess {
                scope.launch { detect(app, cm, cm.activeNetwork, null) }
            }
            .onFailure {
                registered.set(false)
                callback = null
                Log.w(TAG, "registerDefaultNetworkCallback failed", it)
                _status.update { s -> s.copy(lastEvent = "网络监控不可用", lastEventAt = System.currentTimeMillis()) }
            }
    }

    fun refresh(context: Context) {
        val cm = manager ?: context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return
        scope.launch { detect(context.applicationContext, cm, cm.activeNetwork, "手动刷新") }
    }

    fun stop() {
        val cb = callback
        callback = null
        if (registered.compareAndSet(true, false)) {
            cb?.let { runCatching { manager?.unregisterNetworkCallback(it) } }
            manager = null
        }
    }

    private fun handleEvent(app: Context, cm: ConnectivityManager, network: Network?, label: String) {
        val now = System.currentTimeMillis()
        if (now - lastChangeAt < CHANGE_DEBOUNCE_MS) return
        lastChangeAt = now
        runCatching { mobile.Mobile.onNetworkChanged() }
            .onSuccess { Log.i(TAG, "$label: engine caches cleared") }
            .onFailure { Log.w(TAG, "$label: onNetworkChanged failed", it) }
        detectJob?.cancel()
        detectJob = scope.launch { detect(app, cm, network, label) }
    }

    private suspend fun detect(
        app: Context,
        cm: ConnectivityManager,
        network: Network?,
        label: String?
    ) {
        val now = System.currentTimeMillis()
        val target = network ?: cm.activeNetwork
        val linkProps = target?.let { cm.getLinkProperties(it) }
        val v6 = ipv6Available(linkProps)
        val prefix = if (v6 != true) detectNat64Prefix() else null
        val hasDefault = cm.activeNetwork != null
        val status = NetworkStatus(
            ipv6Available = v6,
            nat64Prefix = prefix,
            hasDefaultNetwork = hasDefault,
            lastEvent = label ?: "已检测",
            lastEventAt = now
        )
        _status.value = status
        Log.i(
            TAG,
            "detect: ipv6Available=$v6 nat64Prefix=${prefix ?: "-"} net=$hasDefault event=${label ?: "-"}"
        )
    }

    private fun ipv6Available(lp: LinkProperties?): Boolean? {
        val addrs = lp?.linkAddresses ?: return null
        return addrs.any {
            val address = it.address
            address is Inet6Address &&
                !address.isLinkLocalAddress &&
                !address.isLoopbackAddress &&
                !address.isAnyLocalAddress
        }
    }

    /**
     * RFC 7050：向 DoH 查询 ipv4only.arpa 的 AAAA；若应答内嵌
     * 192.0.0.170 / 192.0.0.171（64:ff9b:: 兼容），低 32 位置零即 NAT64 前缀。
     */
    private suspend fun detectNat64Prefix(): String? = withContext(Dispatchers.IO) {
        val dohUrl = LumineConfig().dns.addr
        if (dohUrl.isBlank()) return@withContext null
        val url = try {
            val sb = java.lang.StringBuilder(dohUrl)
            sb.append(if (dohUrl.contains('?')) '&' else '?')
            sb.append("name=ipv4only.arpa&type=AAAA")
            URL(sb.toString())
        } catch (e: Exception) {
            return@withContext null
        }
        try {
            val conn = url.openConnection() as HttpURLConnection
            conn.connectTimeout = 8000
            conn.readTimeout = 8000
            conn.setRequestProperty("Accept", "application/dns-json")
            if (conn.responseCode !in 200..299) {
                conn.disconnect()
                return@withContext null
            }
            val text = conn.inputStream.bufferedReader().use { it.readText() }
            conn.disconnect()
            val answers = JSONObject(text).optJSONArray("Answer") ?: return@withContext null
            for (i in 0 until answers.length()) {
                val data = answers.optJSONObject(i)?.optString("data") ?: continue
                val aaaa = data.trim()
                if (!aaaa.contains(':')) continue
                try {
                    val bytes = InetAddress.getByName(aaaa).address ?: continue
                    if (bytes.size != 16) continue
                    val v4 = intArrayOf(
                        bytes[12].toInt() and 0xFF,
                        bytes[13].toInt() and 0xFF,
                        bytes[14].toInt() and 0xFF,
                        bytes[15].toInt() and 0xFF
                    )
                    val magic = (v4[0] == 192 && v4[1] == 0 && v4[2] == 0 && (v4[3] == 170 || v4[3] == 171)) ||
                        (v4[0] == 0 && v4[1] == 0 && v4[2] == 0 && v4[3] == 0)
                    if (!magic) continue
                    val prefixBytes = bytes.copyOf(16)
                    prefixBytes[12] = 0
                    prefixBytes[13] = 0
                    prefixBytes[14] = 0
                    prefixBytes[15] = 0
                    return@withContext Inet6Address.getByAddress(null, prefixBytes, 0).hostAddress
                } catch (_: Exception) {
                    // try next answer
                }
            }
            null
        } catch (e: Exception) {
            Log.d(TAG, "detectNat64Prefix failed: ${e.message}")
            null
        }
    }
}
