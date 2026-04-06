package com.moi.lumine

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

data class VpnStatus(
    val phase: String = "idle",
    val message: String = "点此启动服务"
)

object VpnRuntimeState {
    private val _status = MutableStateFlow(VpnStatus())
    val status: StateFlow<VpnStatus> = _status

    fun setStatus(phase: String, message: String) {
        _status.value = VpnStatus(phase = phase, message = message)
    }
}
