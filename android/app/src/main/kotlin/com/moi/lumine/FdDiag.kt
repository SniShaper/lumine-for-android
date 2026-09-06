package com.moi.lumine

import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

object FdDiag {
    fun dump(dir: File, reason: String, note: String = ""): File? = runCatching {
        val outDir = File(dir, "logs").apply { mkdirs() }
        val stamp = SimpleDateFormat("yyyyMMdd-HHmmss-SSS", Locale.US).format(Date())
        val file = File(outDir, "fds_${reason}_$stamp.txt")
        val entries = File("/proc/self/fd").listFiles()
            ?.sortedBy { it.name.toIntOrNull() ?: Int.MAX_VALUE }
            ?: emptyList()
        file.writeText(
            buildString {
                appendLine("reason=$reason note=$note time=$stamp open_fds=${entries.size}")
                appendLine("phase=${VpnRuntimeState.status.value.phase} active=${VpnRuntimeState.isVpnActive.value}")
                entries.forEach { e ->
                    val target = runCatching { e.canonicalPath }.getOrDefault("?")
                    appendLine("${e.name} -> $target")
                }
            }
        )
        file
    }.getOrNull()
}
