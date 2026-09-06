package com.moi.lumine.repository

import android.content.Context
import android.net.Uri
import androidx.core.content.FileProvider
import com.moi.lumine.VpnStatus
import com.moi.lumine.model.LumineConfig
import com.moi.lumine.model.SubscriptionProfile
import com.squareup.moshi.Moshi
import com.squareup.moshi.Types
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import org.json.JSONObject

class ConfigRepository(private val context: Context) {

    private val moshi = Moshi.Builder()
        .add(KotlinJsonAdapterFactory())
        .build()

    private val adapter = moshi.adapter(LumineConfig::class.java).indent("    ")
    private val subscriptionListAdapter = moshi.adapter<List<SubscriptionProfile>>(
        Types.newParameterizedType(List::class.java, SubscriptionProfile::class.java)
    ).indent("    ")
    private val policyAdapter = moshi.adapter(com.moi.lumine.model.Policy::class.java)
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    private fun disabledPrefKey(name: String) = "disabled_rules_$name"

    private fun loadDisabledSnapshots(name: String): JSONObject {
        return try {
            JSONObject(prefs.getString(disabledPrefKey(name), "{}") ?: "{}")
        } catch (e: Exception) {
            JSONObject()
        }
    }

    fun disabledRuleKeys(name: String): Set<String> {
        val snap = loadDisabledSnapshots(name)
        val out = mutableSetOf<String>()
        val it = snap.keys()
        while (it.hasNext()) out.add(it.next())
        return out
    }

    /** 启用/禁用一条规则：禁用项快照到 prefs，写盘时剔除；启用时从快照移除并恢复。 */
    suspend fun setRuleEnabled(name: String, config: LumineConfig, key: String, enabled: Boolean) {
        val snap = loadDisabledSnapshots(name)
        val inDomain = config.domainPolicies.containsKey(key)
        val inIp = config.ipPolicies.containsKey(key)
        if (!inDomain && !inIp) return
        if (enabled) {
            snap.remove(key)
        } else {
            val policy = when {
                inDomain -> config.domainPolicies[key]
                else -> config.ipPolicies[key]
            } ?: return
            val entry = JSONObject()
            entry.put("t", if (inDomain) "d" else "i")
            entry.put("p", policyAdapter.toJson(policy))
            snap.put(key, entry)
        }
        prefs.edit().putString(disabledPrefKey(name), snap.toString()).apply()
        saveConfig(name, config)
    }

    private fun stripDisabled(config: LumineConfig, name: String): LumineConfig {
        val snap = loadDisabledSnapshots(name)
        if (snap.length() == 0) return config
        var dp = config.domainPolicies
        var ip = config.ipPolicies
        val keys = snap.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            val entry = snap.optJSONObject(key) ?: continue
            if (entry.optString("t") == "i") ip = ip - key else dp = dp - key
        }
        return config.copy(domainPolicies = dp, ipPolicies = ip)
    }

    private fun mergeDisabledSnapshots(config: LumineConfig, name: String): LumineConfig {
        val snap = loadDisabledSnapshots(name)
        if (snap.length() == 0) return config
        var dp = config.domainPolicies
        var ip = config.ipPolicies
        val keys = snap.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            val entry = snap.optJSONObject(key) ?: continue
            val policy = try {
                policyAdapter.fromJson(entry.getString("p"))
            } catch (e: Exception) {
                null
            } ?: continue
            if (entry.optString("t") == "i") ip = ip + (key to policy) else dp = dp + (key to policy)
        }
        return config.copy(domainPolicies = dp, ipPolicies = ip)
    }

    suspend fun loadConfig(name: String): LumineConfig? = withContext(Dispatchers.IO) {
        try {
            val file = File(context.filesDir, "$name.json")
            if (!file.exists()) {
                if (name == "config") {
                    val assetJson = try {
                        context.assets.open("config_default.json").bufferedReader().use { it.readText() }
                    } catch (_: Exception) {
                        null
                    }

                    val config = if (assetJson != null) {
                        adapter.fromJson(assetJson) ?: LumineConfig()
                    } else {
                        LumineConfig()
                    }
                    saveConfig("config", config)
                    return@withContext mergeDisabledSnapshots(config, "config")
                }
                return@withContext null
            }
            val parsed = adapter.fromJson(file.readText()) ?: return@withContext null
            return@withContext mergeDisabledSnapshots(parsed, name)
        } catch (e: Exception) {
            e.printStackTrace()
            null
        }
    }

    suspend fun saveConfig(name: String, config: LumineConfig) = withContext(Dispatchers.IO) {
        try {
            val file = File(context.filesDir, "$name.json")
            file.writeText(adapter.toJson(stripDisabled(config, name)))
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    fun listConfigs(): List<String> {
        val files = context.filesDir.listFiles { _, name -> name.endsWith(".json") }
        return files?.map { it.nameWithoutExtension }?.sorted() ?: emptyList()
    }

    fun deleteConfig(name: String) {
        val file = File(context.filesDir, "$name.json")
        if (file.exists()) {
            file.delete()
        }
    }

    fun getSelectedConfigName(): String {
        return prefs.getString(KEY_SELECTED_CONFIG, "config") ?: "config"
    }

    fun setSelectedConfigName(name: String) {
        prefs.edit().putString(KEY_SELECTED_CONFIG, name).apply()
    }

    fun shouldVpnBeRunning(): Boolean {
        return prefs.getBoolean(KEY_VPN_SHOULD_RUN, false)
    }

    fun setVpnShouldRun(shouldRun: Boolean, configName: String? = null) {
        prefs.edit().apply {
            putBoolean(KEY_VPN_SHOULD_RUN, shouldRun)
            if (!configName.isNullOrBlank()) {
                putString(KEY_LAST_RUNNING_CONFIG, configName)
            }
        }.apply()
    }

    fun getLastRunningConfigName(): String {
        return prefs.getString(KEY_LAST_RUNNING_CONFIG, getSelectedConfigName())
            ?: getSelectedConfigName()
    }

    fun getAppRoutingMode(): AppRoutingMode {
        val raw = prefs.getString(KEY_APP_ROUTING_MODE, AppRoutingMode.ALL.value) ?: AppRoutingMode.ALL.value
        return AppRoutingMode.entries.firstOrNull { it.value == raw } ?: AppRoutingMode.ALL
    }

    fun setAppRoutingMode(mode: AppRoutingMode) {
        prefs.edit().putString(KEY_APP_ROUTING_MODE, mode.value).apply()
    }

    fun getAppRoutingPackages(): Set<String> {
        return prefs.getStringSet(KEY_APP_ROUTING_PACKAGES, emptySet()) ?: emptySet()
    }

    fun setAppRoutingPackages(packages: Set<String>) {
        prefs.edit().putStringSet(KEY_APP_ROUTING_PACKAGES, packages).apply()
    }

    suspend fun loadSubscriptions(): List<SubscriptionProfile> = withContext(Dispatchers.IO) {
        try {
            val raw = prefs.getString(KEY_SUBSCRIPTIONS, "[]") ?: "[]"
            subscriptionListAdapter.fromJson(raw).orEmpty()
        } catch (e: Exception) {
            e.printStackTrace()
            emptyList()
        }
    }

    suspend fun saveSubscriptions(subscriptions: List<SubscriptionProfile>) = withContext(Dispatchers.IO) {
        try {
            prefs.edit().putString(KEY_SUBSCRIPTIONS, subscriptionListAdapter.toJson(subscriptions)).apply()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    suspend fun downloadConfig(
        url: String,
        onStatus: (DownloadStatus) -> Unit = {}
    ): LumineConfig = withContext(Dispatchers.IO) {
        onStatus(DownloadStatus(DownloadPhase.Connecting))
        val connection = (URL(url).openConnection() as HttpURLConnection).apply {
            connectTimeout = 15000
            readTimeout = 20000
            instanceFollowRedirects = true
            requestMethod = "GET"
            setRequestProperty("Accept", "application/json, text/plain, */*")
            setRequestProperty("User-Agent", "LumineAndroid/1.0")
        }

        try {
            val code = connection.responseCode
            val stream = if (code in 200..299) connection.inputStream else connection.errorStream
            if (code !in 200..299) {
                val body = stream?.bufferedReader().use { it?.readText().orEmpty() }
                throw IllegalStateException("订阅请求失败: HTTP $code ${body.take(160)}".trim())
            }

            val totalBytes = connection.contentLengthLong.takeIf { it > 0L }
            val output = ByteArrayOutputStream()
            stream?.use { input ->
                val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                var downloadedBytes = 0L
                while (true) {
                    val count = input.read(buffer)
                    if (count < 0) {
                        break
                    }
                    output.write(buffer, 0, count)
                    downloadedBytes += count
                    onStatus(
                        DownloadStatus(
                            phase = DownloadPhase.Downloading,
                            downloadedBytes = downloadedBytes,
                            totalBytes = totalBytes
                        )
                    )
                }
            }

            onStatus(
                DownloadStatus(
                    phase = DownloadPhase.Parsing,
                    downloadedBytes = output.size().toLong(),
                    totalBytes = totalBytes
                )
            )
            val body = output.toString(Charsets.UTF_8.name())
            return@withContext adapter.fromJson(body)
                ?: throw IllegalArgumentException("订阅内容不是有效的 Lumine 配置 JSON")
        } finally {
            connection.disconnect()
        }
    }

    fun generateConfigName(displayName: String): String {
        val base = displayName
            .trim()
            .lowercase()
            .replace(Regex("[^a-z0-9_-]+"), "_")
            .trim('_')
            .ifBlank { "subscription" }

        val existing = listConfigs().toSet()
        if (base !in existing) {
            return base
        }

        var index = 2
        while (true) {
            val candidate = "${base}_$index"
            if (candidate !in existing) {
                return candidate
            }
            index++
        }
    }

    suspend fun exportLogs(
        logs: List<String>,
        status: VpnStatus,
        selectedConfigName: String
    ): ExportedLogFile = withContext(Dispatchers.IO) {
        val exportDir = File(context.cacheDir, "exports").apply { mkdirs() }
        exportDir.listFiles()
            ?.sortedByDescending { it.lastModified() }
            ?.drop(9)
            ?.forEach { it.delete() }

        val now = Date()
        val stamp = SimpleDateFormat("yyyyMMdd-HHmmss-SSS", Locale.US).format(now)
        val exportedAt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(now)
        val fileName = "lumine-log-$stamp.txt"
        val file = File(exportDir, fileName)

        val body = buildString {
            appendLine("Lumine Log Export")
            appendLine("Exported-At: $exportedAt")
            appendLine("Selected-Config: $selectedConfigName")
            appendLine("VPN-Phase: ${status.phase}")
            appendLine("VPN-Message: ${status.message}")
            appendLine("Log-Lines: ${logs.size}")
            appendLine()
            if (logs.isEmpty()) {
                appendLine("[no logs captured]")
            } else {
                logs.forEach { appendLine(it) }
            }
        }

        file.writeText(body)

        val uri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file
        )
        ExportedLogFile(uri = uri, fileName = fileName)
    }

    suspend fun exportConfigJson(name: String, config: LumineConfig): ExportedLogFile = withContext(Dispatchers.IO) {
        val exportDir = File(context.cacheDir, "exports").apply { mkdirs() }
        exportDir.listFiles()
            ?.filter { it.name.startsWith("lumine-config-") }
            ?.sortedByDescending { it.lastModified() }
            ?.drop(9)
            ?.forEach { it.delete() }

        val now = Date()
        val stamp = SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(now)
        val safeName = name.replace(Regex("[^A-Za-z0-9_-]+"), "_")
        val fileName = "lumine-config-$safeName-$stamp.json"
        val file = File(exportDir, fileName)
        file.writeText(adapter.toJson(config))

        val uri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file
        )
        ExportedLogFile(uri = uri, fileName = fileName)
    }

    /**
     * 从 content Uri 导入配置：读文本 -> moshi 解析 -> mobile.CheckConfig 引擎校验。
     * @return 成功返回 (null, 已保存的配置名)；失败返回 (错误信息, null)。
     */
    suspend fun importConfigFromUri(uri: Uri): Pair<String?, String?> = withContext(Dispatchers.IO) {
        val content = try {
            context.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() }
        } catch (e: Exception) {
            null
        }
        if (content.isNullOrBlank()) {
            return@withContext ("配置文件为空或不可读" to null)
        }
        val parsed = try {
            adapter.fromJson(content)
        } catch (e: Exception) {
            null
        }
        if (parsed == null) {
            return@withContext ("配置 JSON 解析失败" to null)
        }
        val engineCheck = try {
            mobile.Mobile.checkConfig(content)
        } catch (e: Throwable) {
            e.message
        }
        if (!engineCheck.isNullOrBlank()) {
            return@withContext ("引擎校验失败：$engineCheck" to null)
        }
        val configName = generateConfigName("import")
        saveConfig(configName, parsed)
        null to configName
    }

    companion object {
        private const val PREFS_NAME = "lumine_prefs"
        private const val KEY_SELECTED_CONFIG = "selected_config_name"
        private const val KEY_SUBSCRIPTIONS = "subscriptions_json"
        private const val KEY_VPN_SHOULD_RUN = "vpn_should_run"
        private const val KEY_LAST_RUNNING_CONFIG = "last_running_config_name"
        private const val KEY_APP_ROUTING_MODE = "app_routing_mode"
        private const val KEY_APP_ROUTING_PACKAGES = "app_routing_packages"
    }
}

enum class AppRoutingMode(val value: String) {
    ALL("all"),
    WHITELIST("whitelist"),
    BYPASS("bypass")
}

data class ExportedLogFile(
    val uri: Uri,
    val fileName: String
)

data class DownloadStatus(
    val phase: DownloadPhase,
    val downloadedBytes: Long = 0L,
    val totalBytes: Long? = null
)

enum class DownloadPhase {
    Connecting,
    Downloading,
    Parsing
}
