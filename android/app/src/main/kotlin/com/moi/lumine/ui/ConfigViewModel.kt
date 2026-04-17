package com.moi.lumine.ui

import android.app.Application
import android.net.Uri
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.moi.lumine.RuntimeLogSnapshot
import com.moi.lumine.VpnRuntimeState
import com.moi.lumine.VpnStatus
import com.moi.lumine.model.LumineConfig
import com.moi.lumine.repository.ExportedLogFile
import com.moi.lumine.model.SubscriptionProfile
import com.moi.lumine.repository.ConfigRepository
import com.moi.lumine.repository.DownloadPhase
import com.moi.lumine.repository.DownloadStatus
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class ConfigViewModel(application: Application) : AndroidViewModel(application) {
    private val repository = ConfigRepository(application)

    private val _currentConfig = MutableStateFlow(LumineConfig())
    val currentConfig: StateFlow<LumineConfig> = _currentConfig

    private val _configList = MutableStateFlow<List<String>>(emptyList())
    val configList: StateFlow<List<String>> = _configList

    private val _selectedConfigName = MutableStateFlow(repository.getSelectedConfigName())
    val selectedConfigName: StateFlow<String> = _selectedConfigName

    private val _selectedConfigDisplayName = MutableStateFlow(_selectedConfigName.value)
    val selectedConfigDisplayName: StateFlow<String> = _selectedConfigDisplayName

    private val _subscriptions = MutableStateFlow<List<SubscriptionProfile>>(emptyList())
    val subscriptions: StateFlow<List<SubscriptionProfile>> = _subscriptions

    private val _subscriptionBusyId = MutableStateFlow<String?>(null)
    val subscriptionBusyId: StateFlow<String?> = _subscriptionBusyId

    private val _isRefreshingAllSubscriptions = MutableStateFlow(false)
    val isRefreshingAllSubscriptions: StateFlow<Boolean> = _isRefreshingAllSubscriptions

    private val _subscriptionMessage = MutableStateFlow<String?>(null)
    val subscriptionMessage: StateFlow<String?> = _subscriptionMessage

    private val _subscriptionImportState = MutableStateFlow(SubscriptionImportState())
    val subscriptionImportState: StateFlow<SubscriptionImportState> = _subscriptionImportState

    private val _isExportingLogs = MutableStateFlow(false)
    val isExportingLogs: StateFlow<Boolean> = _isExportingLogs

    private val _logExportEvent = MutableStateFlow<LogExportEvent?>(null)
    val logExportEvent: StateFlow<LogExportEvent?> = _logExportEvent

    private val _logExportMessage = MutableStateFlow<String?>(null)
    val logExportMessage: StateFlow<String?> = _logExportMessage

    val logSnapshot: StateFlow<RuntimeLogSnapshot> = VpnRuntimeState.logSnapshot
    val isVpnActive: StateFlow<Boolean> = VpnRuntimeState.isVpnActive

    val vpnStatus: StateFlow<VpnStatus> = VpnRuntimeState.status
    val isLogCaptureEnabled: StateFlow<Boolean> = VpnRuntimeState.isLogCaptureEnabled

    private val _editingRuleKey = MutableStateFlow<String?>(null)
    val editingRuleKey: StateFlow<String?> = _editingRuleKey

    init {
        refreshConfigList()
        refreshSubscriptions()
        loadConfig(_selectedConfigName.value)
    }

    fun clearLogs() {
        VpnRuntimeState.clearLogs()
    }

    fun toggleLogCapture() {
        VpnRuntimeState.toggleLogCapture()
    }

    fun exportLogs() {
        if (_isExportingLogs.value) {
            return
        }

        viewModelScope.launch {
            _isExportingLogs.value = true
            try {
                val exported: ExportedLogFile = repository.exportLogs(
                    logs = logSnapshot.value.entries.map { it.raw },
                    status = vpnStatus.value,
                    selectedConfigName = _selectedConfigName.value
                )
                _logExportEvent.value = LogExportEvent(
                    uri = exported.uri,
                    fileName = exported.fileName
                )
            } catch (e: Exception) {
                _logExportMessage.value = e.message ?: "导出日志失败"
            } finally {
                _isExportingLogs.value = false
            }
        }
    }

    fun consumeLogExportEvent() {
        _logExportEvent.value = null
    }

    fun consumeLogExportMessage() {
        _logExportMessage.value = null
    }

    fun reportLogExportMessage(message: String) {
        _logExportMessage.value = message
    }

    fun resetSubscriptionImportState() {
        if (!_subscriptionImportState.value.isRunning) {
            _subscriptionImportState.value = SubscriptionImportState()
        }
    }

    fun setEditingRule(key: String?) {
        _editingRuleKey.value = key
    }

    fun consumeSubscriptionMessage() {
        _subscriptionMessage.value = null
    }

    fun refreshConfigList() {
        viewModelScope.launch {
            val configs = repository.listConfigs()
            _configList.value = configs
        }
    }

    fun refreshSubscriptions() {
        viewModelScope.launch {
            val subscriptions = repository.loadSubscriptions()
                .sortedWith(compareByDescending<SubscriptionProfile> { it.updatedAt }.thenBy { it.name.lowercase() })
            _subscriptions.value = subscriptions
            updateSelectedConfigDisplayName()
        }
    }

    fun loadConfig(name: String) {
        viewModelScope.launch {
            val config = repository.loadConfig(name)
            if (config != null) {
                _currentConfig.value = config
                if (_selectedConfigName.value != name) {
                    _selectedConfigName.value = name
                    repository.setSelectedConfigName(name)
                }
                updateSelectedConfigDisplayName()
            } else if (name != "config") {
                applyConfig("config")
                _subscriptionMessage.value = "配置 $name 不存在，已回退到默认配置"
            }
        }
    }

    fun saveConfig() {
        viewModelScope.launch {
            repository.saveConfig(_selectedConfigName.value, _currentConfig.value)
        }
    }

    fun updateConfig(updated: LumineConfig) {
        _currentConfig.value = updated
    }

    fun applyConfig(name: String) {
        _selectedConfigName.value = name
        repository.setSelectedConfigName(name)
        updateSelectedConfigDisplayName()
        loadConfig(name)
    }

    fun addSubscription(name: String, url: String) {
        val trimmedName = name.trim()
        val trimmedUrl = url.trim()
        if (trimmedName.isEmpty() || trimmedUrl.isEmpty()) {
            _subscriptionImportState.value = SubscriptionImportState(
                stage = SubscriptionImportStage.Error,
                title = "名称和订阅链接都不能为空"
            )
            return
        }
        if (_subscriptions.value.any { it.url.equals(trimmedUrl, ignoreCase = true) }) {
            _subscriptionImportState.value = SubscriptionImportState(
                stage = SubscriptionImportStage.Error,
                title = "这个订阅链接已经添加过了"
            )
            return
        }

        viewModelScope.launch {
            _subscriptionImportState.value = SubscriptionImportState(
                stage = SubscriptionImportStage.Validating,
                title = "正在校验订阅信息",
                detail = trimmedName,
                progress = 0.08f,
                isRunning = true
            )
            _subscriptionBusyId.value = NEW_SUBSCRIPTION_ID
            try {
                val config = repository.downloadConfig(trimmedUrl) { status ->
                    _subscriptionImportState.value = status.toImportState(trimmedName)
                }
                _subscriptionImportState.value = SubscriptionImportState(
                    stage = SubscriptionImportStage.Saving,
                    title = "正在保存配置",
                    detail = trimmedName,
                    progress = 0.94f,
                    isRunning = true
                )
                val configName = repository.generateConfigName(trimmedName)
                repository.saveConfig(configName, config)

                val subscription = SubscriptionProfile(
                    id = configName,
                    name = trimmedName,
                    url = trimmedUrl,
                    configName = configName,
                    updatedAt = System.currentTimeMillis()
                )
                val updated = (_subscriptions.value + subscription)
                    .sortedWith(compareByDescending<SubscriptionProfile> { it.updatedAt }.thenBy { it.name.lowercase() })
                repository.saveSubscriptions(updated)
                _subscriptions.value = updated
                refreshConfigList()
                updateSelectedConfigDisplayName()
                _subscriptionImportState.value = SubscriptionImportState(
                    stage = SubscriptionImportStage.Success,
                    title = "导入完成",
                    detail = trimmedName,
                    progress = 1f,
                    isRunning = false
                )
                _subscriptionMessage.value = "已导入订阅 $trimmedName"
            } catch (e: Exception) {
                _subscriptionImportState.value = SubscriptionImportState(
                    stage = SubscriptionImportStage.Error,
                    title = e.message ?: "导入订阅失败",
                    detail = trimmedName
                )
            } finally {
                _subscriptionBusyId.value = null
            }
        }
    }

    fun refreshSubscription(subscription: SubscriptionProfile) {
        viewModelScope.launch {
            _subscriptionBusyId.value = subscription.id
            try {
                val config = repository.downloadConfig(subscription.url)
                repository.saveConfig(subscription.configName, config)
                val updatedSubscription = subscription.copy(updatedAt = System.currentTimeMillis())
                val updated = _subscriptions.value.map {
                    if (it.id == subscription.id) updatedSubscription else it
                }.sortedWith(compareByDescending<SubscriptionProfile> { it.updatedAt }.thenBy { it.name.lowercase() })
                repository.saveSubscriptions(updated)
                _subscriptions.value = updated
                _subscriptionMessage.value = "已更新 ${subscription.name}"
                if (_selectedConfigName.value == subscription.configName) {
                    loadConfig(subscription.configName)
                }
            } catch (e: Exception) {
                _subscriptionMessage.value = e.message ?: "刷新订阅失败"
            } finally {
                _subscriptionBusyId.value = null
            }
        }
    }

    fun refreshAllSubscriptions() {
        val items = _subscriptions.value
        if (items.isEmpty()) {
            _subscriptionMessage.value = "还没有订阅可以刷新"
            return
        }

        viewModelScope.launch {
            _isRefreshingAllSubscriptions.value = true
            try {
                for (subscription in items) {
                    _subscriptionBusyId.value = subscription.id
                    val config = repository.downloadConfig(subscription.url)
                    repository.saveConfig(subscription.configName, config)
                    _subscriptions.value = _subscriptions.value.map {
                        if (it.id == subscription.id) it.copy(updatedAt = System.currentTimeMillis()) else it
                    }
                }
                val normalized = _subscriptions.value
                    .sortedWith(compareByDescending<SubscriptionProfile> { it.updatedAt }.thenBy { it.name.lowercase() })
                repository.saveSubscriptions(normalized)
                _subscriptions.value = normalized
                if (_selectedConfigName.value in normalized.map { it.configName }) {
                    loadConfig(_selectedConfigName.value)
                }
                _subscriptionMessage.value = "订阅已全部刷新"
            } catch (e: Exception) {
                _subscriptionMessage.value = e.message ?: "批量刷新失败"
            } finally {
                _subscriptionBusyId.value = null
                _isRefreshingAllSubscriptions.value = false
            }
        }
    }

    fun applySubscription(subscription: SubscriptionProfile) {
        applyConfig(subscription.configName)
        _subscriptionMessage.value = "已应用 ${subscription.name}"
    }

    fun deleteSubscription(subscription: SubscriptionProfile) {
        viewModelScope.launch {
            repository.deleteConfig(subscription.configName)
            val updated = _subscriptions.value.filterNot { it.id == subscription.id }
            repository.saveSubscriptions(updated)
            _subscriptions.value = updated
            refreshConfigList()
            if (_selectedConfigName.value == subscription.configName) {
                applyConfig("config")
            } else {
                updateSelectedConfigDisplayName()
            }
            _subscriptionMessage.value = "已删除 ${subscription.name}"
        }
    }

    private fun updateSelectedConfigDisplayName() {
        val match = _subscriptions.value.firstOrNull { it.configName == _selectedConfigName.value }
        _selectedConfigDisplayName.value = match?.name ?: _selectedConfigName.value
    }

    companion object {
        const val NEW_SUBSCRIPTION_ID = "__new_subscription__"
    }
}

data class LogExportEvent(
    val uri: Uri,
    val fileName: String
)

data class SubscriptionImportState(
    val stage: SubscriptionImportStage = SubscriptionImportStage.Idle,
    val title: String = "",
    val detail: String? = null,
    val progress: Float? = null,
    val isRunning: Boolean = false
)

enum class SubscriptionImportStage {
    Idle,
    Validating,
    Connecting,
    Downloading,
    Parsing,
    Saving,
    Success,
    Error
}

private fun DownloadStatus.toImportState(name: String): SubscriptionImportState {
    return when (phase) {
        DownloadPhase.Connecting -> SubscriptionImportState(
            stage = SubscriptionImportStage.Connecting,
            title = "正在连接订阅服务器",
            detail = name,
            progress = 0.16f,
            isRunning = true
        )

        DownloadPhase.Downloading -> {
            val progressValue = totalBytes
                ?.takeIf { it > 0L }
                ?.let { total ->
                    val ratio = downloadedBytes.toFloat() / total.toFloat()
                    (0.18f + ratio.coerceIn(0f, 1f) * 0.62f).coerceIn(0f, 0.8f)
                }
            val detailText = totalBytes
                ?.takeIf { it > 0L }
                ?.let { total -> "已下载 ${formatBytes(downloadedBytes)} / ${formatBytes(total)}" }
                ?: "已下载 ${formatBytes(downloadedBytes)}"
            SubscriptionImportState(
                stage = SubscriptionImportStage.Downloading,
                title = "正在下载配置",
                detail = detailText,
                progress = progressValue,
                isRunning = true
            )
        }

        DownloadPhase.Parsing -> SubscriptionImportState(
            stage = SubscriptionImportStage.Parsing,
            title = "正在解析配置",
            detail = "已接收 ${formatBytes(downloadedBytes)}",
            progress = 0.88f,
            isRunning = true
        )
    }
}

private fun formatBytes(bytes: Long): String {
    if (bytes < 1024L) {
        return "${bytes} B"
    }
    if (bytes < 1024L * 1024L) {
        return String.format("%.1f KB", bytes / 1024f)
    }
    return String.format("%.2f MB", bytes / (1024f * 1024f))
}
