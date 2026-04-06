package com.moi.lumine.repository

import android.content.Context
import com.moi.lumine.model.LumineConfig
import com.moi.lumine.model.SubscriptionProfile
import com.squareup.moshi.Moshi
import com.squareup.moshi.Types
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL

class ConfigRepository(private val context: Context) {

    private val moshi = Moshi.Builder()
        .add(KotlinJsonAdapterFactory())
        .build()

    private val adapter = moshi.adapter(LumineConfig::class.java).indent("    ")
    private val subscriptionListAdapter = moshi.adapter<List<SubscriptionProfile>>(
        Types.newParameterizedType(List::class.java, SubscriptionProfile::class.java)
    ).indent("    ")
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

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
                    return@withContext config
                }
                return@withContext null
            }
            adapter.fromJson(file.readText())
        } catch (e: Exception) {
            e.printStackTrace()
            null
        }
    }

    suspend fun saveConfig(name: String, config: LumineConfig) = withContext(Dispatchers.IO) {
        try {
            val file = File(context.filesDir, "$name.json")
            file.writeText(adapter.toJson(config))
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

    suspend fun downloadConfig(url: String): LumineConfig = withContext(Dispatchers.IO) {
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
            val body = stream?.bufferedReader().use { it?.readText().orEmpty() }
            if (code !in 200..299) {
                throw IllegalStateException("订阅请求失败: HTTP $code ${body.take(160)}".trim())
            }

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

    companion object {
        private const val PREFS_NAME = "lumine_prefs"
        private const val KEY_SELECTED_CONFIG = "selected_config_name"
        private const val KEY_SUBSCRIPTIONS = "subscriptions_json"
    }
}
