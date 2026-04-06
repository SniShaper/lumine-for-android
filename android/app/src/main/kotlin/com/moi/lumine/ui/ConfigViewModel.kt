package com.moi.lumine.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.moi.lumine.VpnRuntimeState
import com.moi.lumine.VpnStatus
import com.moi.lumine.model.LumineConfig
import com.moi.lumine.model.SubscriptionProfile
import com.moi.lumine.repository.ConfigRepository
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mobile.Mobile

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

    private val _logs = MutableStateFlow<List<String>>(emptyList())
    val logs: StateFlow<List<String>> = _logs

    private val _isVpnActive = MutableStateFlow(false)
    val isVpnActive: StateFlow<Boolean> = _isVpnActive

    val vpnStatus: StateFlow<VpnStatus> = VpnRuntimeState.status

    private val _editingRuleKey = MutableStateFlow<String?>(null)
    val editingRuleKey: StateFlow<String?> = _editingRuleKey

    init {
        refreshConfigList()
        refreshSubscriptions()
        loadConfig(_selectedConfigName.value)
        startStatusPolling()
        startLogPolling()
    }

    private fun startStatusPolling() {
        viewModelScope.launch(Dispatchers.IO) {
            while (true) {
                val running = Mobile.isRunning()
                withContext(Dispatchers.Main) {
                    _isVpnActive.value = running
                    if (running && vpnStatus.value.phase != "running") {
                        VpnRuntimeState.setStatus("running", "代理运行中")
                    } else if (!running && vpnStatus.value.phase == "running") {
                        VpnRuntimeState.setStatus("idle", "点此启动服务")
                    }
                }
                delay(1000)
            }
        }
    }

    private fun startLogPolling() {
        viewModelScope.launch(Dispatchers.IO) {
            while (true) {
                val running = Mobile.isRunning()
                var updatedLogs: List<String>? = null
                val newLogs = Mobile.getLogs()
                if (newLogs.isNotBlank()) {
                    val lines = newLogs
                        .lineSequence()
                        .map { it.trimEnd() }
                        .filter { it.isNotBlank() }
                        .toList()
                    if (lines.isNotEmpty()) {
                        updatedLogs = (_logs.value + lines).takeLast(1000)
                    }
                }
                val logsToApply = updatedLogs
                withContext(Dispatchers.Main) {
                    _isVpnActive.value = running
                    if (logsToApply != null) {
                        _logs.value = logsToApply
                    }
                }
                delay(1000)
            }
        }
    }

    fun setVpnActive(active: Boolean) {
        _isVpnActive.value = active
    }

    fun clearLogs() {
        _logs.value = emptyList()
    }

    fun setEditingRule(key: String?) {
        _editingRuleKey.value = key
    }

    fun consumeSubscriptionMessage() {
        _subscriptionMessage.value = null
    }

    fun refreshConfigList() {
        viewModelScope.launch(Dispatchers.IO) {
            val configs = repository.listConfigs()
            withContext(Dispatchers.Main) {
                _configList.value = configs
            }
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
            _subscriptionMessage.value = "名称和订阅链接都不能为空"
            return
        }
        if (_subscriptions.value.any { it.url.equals(trimmedUrl, ignoreCase = true) }) {
            _subscriptionMessage.value = "这个订阅链接已经添加过了"
            return
        }

        viewModelScope.launch {
            _subscriptionBusyId.value = NEW_SUBSCRIPTION_ID
            try {
                val config = repository.downloadConfig(trimmedUrl)
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
                _subscriptionMessage.value = "已导入订阅 $trimmedName"
            } catch (e: Exception) {
                _subscriptionMessage.value = e.message ?: "导入订阅失败"
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
