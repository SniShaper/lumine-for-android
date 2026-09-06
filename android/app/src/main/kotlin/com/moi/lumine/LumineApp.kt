package com.moi.lumine

import android.app.Application
import android.util.Log
import com.moi.lumine.keepalive.KeepAlive
import com.moi.lumine.network.NetworkMonitor
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import mobile.Mobile

class LumineApp : Application() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun onCreate() {
        super.onCreate()
        installCrashLogHandler()
        enableFdsanWarnIfFlagged()

        // 开启过代理才保活：调度闹钟/JobScheduler/WorkManager
        if (KeepAlive.shouldRun(this)) {
            KeepAlive.scheduleAll(this)
        }

        // 网络环境监控（IPv6/NAT64 检测 + 引擎缓存刷新），进程级单例
        NetworkMonitor.start(this)

        startWatchdog()
    }

    // 未捕获 Java 异常落盘到 logs/crash_*.txt，随会话日志目录一并导出
    private fun enableFdsanWarnIfFlagged() {
        val flag = File(filesDir, "fdsan_warn").exists()
        val debuggable = (applicationInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0
        if (flag || debuggable) {
            Log.w("LumineApp", "fdsan warn-always enabled (flag=$flag debuggable=$debuggable)")
            runCatching { Mobile.setFdsanWarnOnly(true) }
                .onFailure { Log.e("LumineApp", "setFdsanWarnOnly failed: ${it.message}") }
        }
    }

    private fun installCrashLogHandler() {
        val defaultHandler = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, throwable ->
            runCatching {
                val dir = File(filesDir, "logs").apply { mkdirs() }
                val stamp = SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(Date())
                val crashFile = File(dir, "crash_$stamp.txt")
                val recent = VpnRuntimeState.logSnapshot.value.entries.takeLast(150)
                val fdSnap = FdDiag.dump(filesDir, "java_crash", thread.name)
                crashFile.writeText(
                    buildString {
                        appendLine("Crash time: " + SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date()))
                        appendLine("Thread: ${thread.name}")
                        appendLine("Phase: ${VpnRuntimeState.status.value.phase}")
                        appendLine("Active: ${VpnRuntimeState.isVpnActive.value}")
                        fdSnap?.let { appendLine("Fd snapshot: ${it.name}") }
                        appendLine()
                        appendLine("Recent log lines: ${recent.size}")
                        recent.forEach { appendLine(it.raw) }
                        appendLine()
                        appendLine("Stack trace:")
                        appendLine(Log.getStackTraceString(throwable))
                    }
                )
            }
            defaultHandler?.uncaughtException(thread, throwable)
        }
    }

    // 进程存活但服务已死 → 拉起（每 10 秒检查，仅代理开启时）
    private fun startWatchdog() {
        scope.launch {
            while (isActive) {
                delay(10_000L)
                if (!KeepAlive.shouldRun(this@LumineApp)) continue
                if (!LumineVpnService.isServiceRunning) {
                    Log.w("LumineApp", "进程存活但服务已死，拉起代理服务")
                    KeepAlive.tryRestart(this@LumineApp)
                }
            }
        }
    }
}
