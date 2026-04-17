package com.moi.lumine

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

data class VpnStatus(
    val phase: String = "idle",
    val message: String = "点此启动服务"
)

enum class RuntimeLogLevel {
    Info,
    Error,
    Debug,
    Other
}

data class RuntimeLogEntry(
    val id: Long,
    val raw: String,
    val timestamp: String?,
    val tag: String?,
    val level: RuntimeLogLevel,
    val message: String
)

data class RuntimeLogSnapshot(
    val entries: List<RuntimeLogEntry> = emptyList(),
    val totalCount: Int = 0,
    val infoCount: Int = 0,
    val errorCount: Int = 0,
    val debugCount: Int = 0
)

object VpnRuntimeState {
    private const val MAX_LOG_ENTRIES = 1000
    private val TIMESTAMP_REGEX = Regex("""^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\s+""")
    private val TAG_REGEX = Regex("""^\[[^\]]+\]""")

    private val _status = MutableStateFlow(VpnStatus())
    val status: StateFlow<VpnStatus> = _status.asStateFlow()

    private val _isVpnActive = MutableStateFlow(false)
    val isVpnActive: StateFlow<Boolean> = _isVpnActive.asStateFlow()

    private val _isLogCaptureEnabled = MutableStateFlow(false)
    val isLogCaptureEnabled: StateFlow<Boolean> = _isLogCaptureEnabled.asStateFlow()

    private val _logSnapshot = MutableStateFlow(RuntimeLogSnapshot())
    val logSnapshot: StateFlow<RuntimeLogSnapshot> = _logSnapshot.asStateFlow()

    private val logLock = Any()
    private val logBuffer = ArrayDeque<RuntimeLogEntry>(MAX_LOG_ENTRIES)
    private var nextLogId = 0L
    private var infoCount = 0
    private var errorCount = 0
    private var debugCount = 0

    fun setStatus(phase: String, message: String) {
        _status.value = VpnStatus(phase = phase, message = message)
    }

    fun setActive(active: Boolean) {
        _isVpnActive.value = active
    }

    fun setLogCaptureEnabled(enabled: Boolean) {
        _isLogCaptureEnabled.value = enabled
    }

    fun toggleLogCapture() {
        _isLogCaptureEnabled.value = !_isLogCaptureEnabled.value
    }

    fun appendLogs(lines: List<String>) {
        if (!_isLogCaptureEnabled.value || lines.isEmpty()) {
            return
        }

        synchronized(logLock) {
            lines.forEach { raw ->
                val entry = parseLogEntry(raw)
                logBuffer.addLast(entry)
                incrementCount(entry.level)

                while (logBuffer.size > MAX_LOG_ENTRIES) {
                    val removed = logBuffer.removeFirst()
                    decrementCount(removed.level)
                }
            }
            emitLogSnapshotLocked()
        }
    }

    fun clearLogs() {
        synchronized(logLock) {
            logBuffer.clear()
            nextLogId = 0L
            infoCount = 0
            errorCount = 0
            debugCount = 0
            _logSnapshot.value = RuntimeLogSnapshot()
        }
    }

    private fun emitLogSnapshotLocked() {
        _logSnapshot.value = RuntimeLogSnapshot(
            entries = logBuffer.toList(),
            totalCount = logBuffer.size,
            infoCount = infoCount,
            errorCount = errorCount,
            debugCount = debugCount
        )
    }

    private fun parseLogEntry(raw: String): RuntimeLogEntry {
        val trimmedRaw = raw.trim()
        val timestampMatch = TIMESTAMP_REGEX.find(trimmedRaw)
        val timestamp = timestampMatch?.value?.trim()
        val withoutTimestamp = if (timestampMatch != null) {
            trimmedRaw.removePrefix(timestampMatch.value).trim()
        } else {
            trimmedRaw
        }

        val tagMatch = TAG_REGEX.find(withoutTimestamp)
        val tag = tagMatch?.value
        val message = if (tagMatch != null) {
            withoutTimestamp.removePrefix(tagMatch.value).trim()
        } else {
            withoutTimestamp
        }

        return RuntimeLogEntry(
            id = nextLogId++,
            raw = raw,
            timestamp = timestamp,
            tag = tag,
            level = detectLogLevel(message),
            message = message
        )
    }

    private fun detectLogLevel(message: String): RuntimeLogLevel {
        return when {
            message.contains("ERROR", ignoreCase = true) ||
                message.contains("failed", ignoreCase = true) -> RuntimeLogLevel.Error

            message.contains("DEBUG", ignoreCase = true) ||
                message.contains("closed", ignoreCase = true) -> RuntimeLogLevel.Debug

            message.contains("INFO", ignoreCase = true) ||
                message.contains("started", ignoreCase = true) ||
                message.contains("CONNECT") -> RuntimeLogLevel.Info

            else -> RuntimeLogLevel.Other
        }
    }

    private fun incrementCount(level: RuntimeLogLevel) {
        when (level) {
            RuntimeLogLevel.Info -> infoCount += 1
            RuntimeLogLevel.Error -> errorCount += 1
            RuntimeLogLevel.Debug -> debugCount += 1
            RuntimeLogLevel.Other -> Unit
        }
    }

    private fun decrementCount(level: RuntimeLogLevel) {
        when (level) {
            RuntimeLogLevel.Info -> infoCount -= 1
            RuntimeLogLevel.Error -> errorCount -= 1
            RuntimeLogLevel.Debug -> debugCount -= 1
            RuntimeLogLevel.Other -> Unit
        }
    }
}
