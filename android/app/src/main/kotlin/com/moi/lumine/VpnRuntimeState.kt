package com.moi.lumine

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

data class VpnStatus(
    val phase: String = "idle",
    val message: String = "点此启动服务"
)

object VpnRuntimeState {
    private val _status = MutableStateFlow(VpnStatus())
    val status: StateFlow<VpnStatus> = _status.asStateFlow()

    private val _isVpnActive = MutableStateFlow(false)
    val isVpnActive: StateFlow<Boolean> = _isVpnActive.asStateFlow()

    private val _logs = MutableStateFlow<List<String>>(emptyList())
    val logs: StateFlow<List<String>> = _logs.asStateFlow()

    fun setStatus(phase: String, message: String) {
        _status.value = VpnStatus(phase = phase, message = message)
    }

    fun setActive(active: Boolean) {
        _isVpnActive.value = active
    }

    fun appendLogs(lines: List<String>) {
        if (lines.isEmpty()) {
            return
        }
        _logs.value = (_logs.value + lines).takeLast(1000)
    }

    fun clearLogs() {
        _logs.value = emptyList()
    }
}
