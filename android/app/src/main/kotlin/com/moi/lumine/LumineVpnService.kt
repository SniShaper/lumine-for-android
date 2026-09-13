package com.moi.lumine

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.os.SystemClock
import android.util.Log
import com.moi.lumine.keepalive.KeepAlive
import com.moi.lumine.repository.AppRoutingMode
import com.moi.lumine.repository.ConfigRepository
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.concurrent.Executors
import mobile.Mobile // This will be available after gomobile bind

class LumineVpnService : VpnService() {

    // vpnInterface/coreTunFd/configName 会在主线程与 lifecycle 执行线程间读写，
    // 必须 @Volatile；vpnInterface 的判空-使用-清空统一走 transitionLock 互斥。
    @Volatile private var vpnInterface: ParcelFileDescriptor? = null
    @Volatile private var coreTunFd: Int? = null
    @Volatile private var configName: String = "config" // Default config name
    private val transitionExecutor = Executors.newSingleThreadExecutor { r ->
        Thread(r, "lumine-lifecycle").apply { isDaemon = false }
    }
    private val serviceScope = CoroutineScope(SupervisorJob() + transitionExecutor.asCoroutineDispatcher())
    private val repository by lazy { ConfigRepository(applicationContext) }
    private val transitionLock = Any()
    // logPumpJob/watchdogJob 会在 lifecycle 执行线程与主线程（onDestroy/
    // onTaskRemoved）间读写，必须 @Volatile 保证可见性，否则重复的日志泵/
    // 看门狗可能同时运行（日志重复/状态错乱）。
    @Volatile private var logPumpJob: Job? = null
    @Volatile private var watchdogJob: Job? = null
    @Volatile private var isStarting = false
    @Volatile private var isStopping = false
    @Volatile private var coreStarted = false
    @Volatile private var coreOwnsTunFd = false
    @Volatile private var pendingStopRequested = false
    @Volatile private var coreStopIssued = false
    @Volatile private var suppressAutoRestart = false
    @Volatile private var lastWatchdogRecoveryAt = 0L
    private var watchdogRecoveryAttempts = 0 // 仅 watchdog 协程内读写

    override fun onCreate() {
        super.onCreate()
        isServiceRunning = true
        if (repository.recordCrashRestart()) {
            suppressAutoRestart = true
            Log.w("LumineVpn", "Too many rapid restarts, suppressing auto-recovery")
            // FdDiag.dump 涉及磁盘 IO，移出主线程
            serviceScope.launch {
                FdDiag.dump(filesDir, "restart", "auto-recovery suppressed")
            }
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val action = intent?.action
        if (action == ACTION_STOP) {
            // 经 startForegroundService 拉起的服务必须先 startForeground 再
            // 停止，否则触发 ForegroundServiceDidNotStartInTimeException。
            startForeground(NOTIFICATION_ID, buildNotification("正在停止代理"))
            repository.resetCrashCounter()
            suppressAutoRestart = false
            repository.setVpnShouldRun(false)
            VpnRuntimeState.setStatus("stopping", "正在停止代理")
            stopVpn()
            return START_NOT_STICKY
        }

        val requestedConfig = intent?.getStringExtra(EXTRA_CONFIG_NAME)?.takeIf { it.isNotBlank() }
        val shouldRecover = requestedConfig == null && repository.shouldVpnBeRunning()
        val targetConfig = requestedConfig ?: if (shouldRecover) repository.getLastRunningConfigName() else null

        if (requestedConfig != null) {
            repository.resetCrashCounter()
            suppressAutoRestart = false
        }

        if (shouldRecover && suppressAutoRestart) {
            Log.w("LumineVpn", "Suppressing auto-recovery after repeated crashes")
            // stopSelf 前先满足前台服务时限要求（startForegroundService 拉起的场景）。
            startForeground(NOTIFICATION_ID, buildNotification("已停止自动恢复"))
            repository.setVpnShouldRun(false)
            VpnRuntimeState.setActive(false)
            VpnRuntimeState.setStatus("idle", "多次异常退出，已停止自动恢复代理")
            stopSelf()
            return START_NOT_STICKY
        }

        // 恢复路径：VPN 授权丢失（划掉重启/系统重置）时，拉起主界面重新授权，
        // 不要在这里 startVpn()，否则 establish() 抛 SecurityException 会清掉保活标志
        if (shouldRecover && VpnService.prepare(this) != null) {
            Log.i("LumineVpn", "VPN permission lost, requesting re-authorization")
            // startForegroundService 起的服务必须在 5 秒内 startForeground，否则系统杀服务
            startForeground(NOTIFICATION_ID, buildNotification("等待 VPN 授权"))
            VpnRuntimeState.setStatus("authorizing", "需要重新授权 VPN 权限")
            requestVpnPermissionFromUi()
            return START_STICKY
        }

        if (targetConfig == null) {
            Log.i("LumineVpn", "Ignoring sticky restart without persisted running state")
            // 该路径可能由 startForegroundService 触达（恢复竞态），同样需要
            // 先 startForeground 以免超过前台服务时限。
            startForeground(NOTIFICATION_ID, buildNotification("待机"))
            if (!Mobile.isRunning() && VpnRuntimeState.status.value.phase != "error") {
                VpnRuntimeState.setActive(false)
                VpnRuntimeState.setStatus("idle", "点此启动服务")
            }
            return START_STICKY
        }

        configName = targetConfig
        repository.setSelectedConfigName(configName)
        repository.setVpnShouldRun(true, configName)
        if (shouldRecover) {
            VpnRuntimeState.setStatus("starting", "正在恢复代理")
            Log.i("LumineVpn", "Recovering VPN after service restart with config: $configName")
        }
        startVpn()
        return START_STICKY
    }

    private fun requestVpnPermissionFromUi() {
        try {
            val intent = Intent(this, MainActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                putExtra(EXTRA_REQUEST_VPN_PERMISSION, true)
            }
            startActivity(intent)
        } catch (e: Exception) {
            Log.w("LumineVpn", "无法打开授权页面: ${e.message}")
        }
    }

    private fun startVpn() {
        synchronized(transitionLock) {
            if (isStarting || isStopping || coreStarted || vpnInterface != null || coreTunFd != null) {
                Log.i("LumineVpn", "Ignoring duplicate start request")
                return
            }
            isStarting = true
            pendingStopRequested = false
            coreStopIssued = false
        }

        try {
            VpnRuntimeState.clearLogs()
            VpnRuntimeState.setActive(false)
            VpnRuntimeState.setStatus("starting", "正在建立 VPN")
            startForeground(NOTIFICATION_ID, buildNotification("正在启动代理"))

            val builder = Builder()
                .setSession("Lumine")
                .setMtu(1500)
                .addAddress("172.19.0.1", 30) // Virtual IP
                .addAddress("fd66:6c75:6d69::1", 64) // Virtual IPv6
                .addDnsServer("172.19.0.2")   // Lumine hijacked DNS
                .addRoute("0.0.0.0", 0)       // Global IPv4 proxy
                .addRoute("::", 0)            // Global IPv6 proxy

            // Keep the app's own sockets out of the VPN to avoid proxy self-loops.
            applyAppRouting(builder)

            // establish() 结果局部捕获后再使用，消除 check-then-act 竞态
            // （旧代码的 vpnInterface!! 可能被并发置空导致 NPE）。
            val established = builder.establish()
            vpnInterface = established

            if (established != null) {
                val pfd = established
                val fd = pfd.fd
                Log.i("LumineVpn", "Established TUN FD: $fd")
                VpnRuntimeState.setStatus("starting", "VPN 已建立，正在启动核心")

                serviceScope.launch {
                    try {
                        ensureConfigFile(configName)
                        if (consumePendingStopRequest()) {
                            closePendingTunFd()
                            VpnRuntimeState.setActive(false)
                            VpnRuntimeState.setStatus("idle", "点此启动服务")
                            return@launch
                        }

                        Mobile.setWorkingDir(filesDir.absolutePath)
                        synchronized(transitionLock) {
                            if (pendingStopRequested) {
                                closePendingTunFd()
                                VpnRuntimeState.setActive(false)
                                VpnRuntimeState.setStatus("idle", "点此启动服务")
                                return@launch
                            }
                            coreOwnsTunFd = true
                        }
                        val error = Mobile.startLumine(fd.toLong(), configName)
                        // Go 引擎在内部 dup 了一份无 fdsan 所有权的 fd；此处由 Java 关闭原始 fd，
                        // 确保 Android 侧的所有权表项被正确清除（避免 fd 号复用触发 fdsan）。
                        closePendingTunFd()
                        if (error.isNotEmpty()) {
                            coreOwnsTunFd = false
                            Log.e("LumineVpn", "Go core failed: $error")
                            updateNotification("启动失败: $error")
                            VpnRuntimeState.setActive(false)
                            VpnRuntimeState.setStatus("error", "启动失败: $error")
                            repository.setVpnShouldRun(false)
                            stopVpn()
                        } else {
                            coreStarted = true
                            Log.i("LumineVpn", "Lumine started successfully")
                            FdDiag.dump(filesDir, "start", "original_fd=$fd engine=dup-close")
                            VpnRuntimeState.setActive(true)
                            VpnRuntimeState.setStatus("running", "代理运行中")
                            updateNotification("代理运行中")
                            startLogPump()
                            startWatchdog()
                            if (consumePendingStopRequest()) {
                                stopVpn()
                            }
                        }
                    } catch (e: Exception) {
                        closePendingTunFd()
                        Log.e("LumineVpn", "Failed to initialize Go core", e)
                        VpnRuntimeState.setActive(false)
                        VpnRuntimeState.setStatus("error", "核心初始化失败")
                        repository.setVpnShouldRun(false)
                        stopVpn()
                    } finally {
                        isStarting = false
                    }
                }
            } else {
                isStarting = false
                VpnRuntimeState.setActive(false)
                VpnRuntimeState.setStatus("error", "VPN 建立失败")
                repository.setVpnShouldRun(false)
                serviceScope.launch {
                    stopServiceShell()
                }
            }
        } catch (e: Exception) {
            Log.e("LumineVpn", "Failed to start VPN", e)
            isStarting = false
            VpnRuntimeState.setActive(false)
            VpnRuntimeState.setStatus("error", "启动 VPN 失败")
            repository.setVpnShouldRun(false)
            serviceScope.launch {
                stopServiceShell()
            }
        }
    }

    private fun stopVpn() {
        val stopShellImmediately = synchronized(transitionLock) {
            if (isStopping) {
                Log.i("LumineVpn", "Ignoring duplicate stop request")
                return@synchronized false
            }
            if (!isStarting && !coreStarted && vpnInterface == null && coreTunFd == null) {
                true
            } else {
                pendingStopRequested = true
                isStopping = true
                false
            }
        }
        if (stopShellImmediately) {
            serviceScope.launch {
                stopWatchdog()
                stopServiceShell()
            }
            return
        }
        serviceScope.launch {
            try {
                stopWatchdog()
                stopLogPump()
                performCoreShutdownIfNeeded()
                FdDiag.dump(filesDir, "stop", "engine stopped")
                pendingStopRequested = false
                VpnRuntimeState.setActive(false)

                val tun = synchronized(transitionLock) {
                    val t = vpnInterface
                    vpnInterface = null
                    t
                }
                runCatching { tun?.close() }

                withContext(Dispatchers.Main) {
                    runCatching {
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                            stopForeground(STOP_FOREGROUND_REMOVE)
                        } else {
                            @Suppress("DEPRECATION")
                            stopForeground(true)
                        }
                    }
                    stopSelf()
                    if (VpnRuntimeState.status.value.phase != "error") {
                        VpnRuntimeState.setStatus("idle", "点此启动服务")
                    }
                }
            } finally {
                isStarting = false
                isStopping = false
            }
        }
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        super.onTaskRemoved(rootIntent)
        // 用户要求：划掉后台 → 完全停止服务（停核心 + 移除前台通知，UI/通知状态一致）。
        // 保活组件（无障碍/闹钟/JobScheduler）随后拉起新实例，recover 路径会重新校验 VPN 授权。
        if (KeepAlive.shouldRun(this)) {
            KeepAlive.scheduleAll(this)
            stopWatchdog()
            stopLogPump()
            // Mobile.stopLumine 是阻塞 FFI，移出主线程避免 ANR
            serviceScope.launch {
                performCoreShutdownIfNeeded()
            }
            runCatching {
                stopForeground(STOP_FOREGROUND_REMOVE)
            }
            stopSelf()
            VpnRuntimeState.setActive(false)
            VpnRuntimeState.setStatus("idle", "点此启动服务")
        }
    }

    override fun onDestroy() {
        isServiceRunning = false
        stopWatchdog()
        stopLogPump()
        // Mobile.stopLumine 是阻塞 FFI，不能在主线程执行；serviceScope 即将
        // cancel，改由 lifecycle 执行器排队执行（shutdown 不中断已排队任务），
        // 确保 TUN fd 与引擎资源在服务销毁后仍被正确释放。
        transitionExecutor.execute {
            performCoreShutdownIfNeeded()
        }
        serviceScope.cancel()
        transitionExecutor.shutdown()
        pendingStopRequested = false
        VpnRuntimeState.setActive(false)
        runCatching { vpnInterface?.close() }
        vpnInterface = null
        isStarting = false
        isStopping = false
        if (VpnRuntimeState.status.value.phase != "error") {
            VpnRuntimeState.setStatus("idle", "点此启动服务")
        }
        super.onDestroy()
    }

    private fun applyAppRouting(builder: Builder) {
        val mode = repository.getAppRoutingMode()
        val packages = repository.getAppRoutingPackages()
            .filter { it.isNotBlank() && it != packageName }
            .distinct()
        when (mode) {
            AppRoutingMode.WHITELIST -> {
                if (packages.isEmpty()) {
                    builder.addDisallowedApplication(packageName)
                    return
                }
                var added = 0
                packages.forEach { pkg ->
                    runCatching { builder.addAllowedApplication(pkg) }
                        .onSuccess { added += 1 }
                }
                if (added == 0) {
                    // 白名单全部包名无效：静默继续会变成"全量 VPN"或空 VPN 的
                    // 错误行为，视为配置错误并中止启动（K13）。
                    Log.e("LumineVpn", "Whitelist app routing has no valid packages: $packages")
                    throw IllegalStateException("应用分流白名单中的包名全部无效，已中止启动")
                }
            }
            AppRoutingMode.BYPASS -> {
                (packages + packageName).forEach { pkg ->
                    runCatching { builder.addDisallowedApplication(pkg) }
                }
            }
            AppRoutingMode.ALL -> builder.addDisallowedApplication(packageName)
        }
    }

    private fun ensureConfigFile(name: String) {
        val target = File(filesDir, "$name.json")
        if (target.exists()) {
            return
        }

        if (name == "config") {
            // 资产拷贝走临时文件 + 原子重命名，避免半写损坏默认配置（K6）
            val tmp = File(filesDir, "$name.json.tmp")
            assets.open("config_default.json").use { input ->
                tmp.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
            if (!tmp.renameTo(target)) {
                target.delete()
                if (!tmp.renameTo(target)) {
                    tmp.delete()
                    throw IllegalStateException("Failed to persist default config atomically")
                }
            }
            Log.i("LumineVpn", "Created default config at ${target.absolutePath}")
            return
        }

        throw IllegalStateException("Config file not found: $name.json")
    }

    private fun startLogPump() {
        if (logPumpJob?.isActive == true) {
            return
        }
        logPumpJob = serviceScope.launch {
            while (isActive) {
                publishPendingLogs()
                delay(300)
            }
        }
    }

    private fun stopLogPump() {
        logPumpJob?.cancel()
        logPumpJob = null
    }

    private fun startWatchdog() {
        if (watchdogJob?.isActive == true) {
            return
        }
        watchdogJob = serviceScope.launch {
            while (isActive) {
                delay(WATCHDOG_INTERVAL_MS)

                if (!repository.shouldVpnBeRunning() || isStarting || isStopping || pendingStopRequested) {
                    continue
                }

                val coreRunning = runCatching { Mobile.isRunning() }.getOrDefault(false)
                if (coreRunning) {
                    continue
                }

                val now = SystemClock.elapsedRealtime()
                // 上次恢复距今已稳定运行超过阈值 → 复位退避/放弃计数
                if (now - lastWatchdogRecoveryAt > WATCHDOG_STABLE_RESET_MS) {
                    watchdogRecoveryAttempts = 0
                }
                if (now - lastWatchdogRecoveryAt < WATCHDOG_RECOVERY_COOLDOWN_MS) {
                    continue
                }
                lastWatchdogRecoveryAt = now

                Log.w("LumineVpn", "Watchdog detected core/service desync, restarting VPN")
                VpnRuntimeState.setActive(false)
                VpnRuntimeState.setStatus("starting", "检测到核心退出，正在恢复")
                recoverVpnFromWatchdog()
            }
        }
    }

    private fun stopWatchdog() {
        watchdogJob?.cancel()
        watchdogJob = null
    }

    private fun publishPendingLogs() {
        val newLogs = Mobile.getLogs()
        if (newLogs.isBlank()) {
            return
        }
        val lines = newLogs
            .lineSequence()
            .map { it.trimEnd() }
            .filter { it.isNotBlank() }
            .toList()
        VpnRuntimeState.appendLogs(lines)
    }

    private fun consumePendingStopRequest(): Boolean {
        synchronized(transitionLock) {
            if (!pendingStopRequested) {
                return false
            }
            pendingStopRequested = false
            return true
        }
    }

    private fun closePendingTunFd() {
        // 与 startVpn/stopVpn 的读取路径互斥：判空-置空-关闭原子化，避免
        // 并发下重复 close 或读到半更新状态。
        val pfd = synchronized(transitionLock) {
            coreTunFd = null
            val pending = vpnInterface
            vpnInterface = null
            pending
        }
        // 引擎在 Go 侧 dup 接管了它自己的 fd；这里的 ParcelFileDescriptor 始终由 Java
        // 通过正式 close() 释放，确保 Android fdsan 所有权表项被正确清除。
        runCatching { pfd?.close() }
    }

    private suspend fun recoverVpnFromWatchdog() {
        val attempts = ++watchdogRecoveryAttempts

        // 接入既有崩溃计数器：短窗口内频繁恢复视同崩溃循环，直接放弃。
        if (repository.recordCrashRestart()) {
            giveUpWatchdogRecovery("多次快速恢复，已停止自动恢复代理")
            return
        }
        if (attempts > WATCHDOG_MAX_RECOVERY_ATTEMPTS) {
            giveUpWatchdogRecovery("自动恢复次数达到上限，已停止恢复")
            return
        }

        val claimed = synchronized(transitionLock) {
            if (isStarting || isStopping || pendingStopRequested) {
                false
            } else {
                isStopping = true
                true
            }
        }
        if (!claimed) {
            return
        }

        try {
            stopLogPump()
            performCoreShutdownIfNeeded()

            val tun = synchronized(transitionLock) {
                val t = vpnInterface
                vpnInterface = null
                t
            }
            runCatching { tun?.close() }

            closePendingTunFd()
            coreStarted = false
            coreOwnsTunFd = false
            pendingStopRequested = false
        } finally {
            isStarting = false
            isStopping = false
        }

        if (!repository.shouldVpnBeRunning()) {
            stopServiceShell()
            return
        }

        // 指数退避：5s→10s→…→上限 5min。退避在释放 isStopping 之后进行，
        // 用户主动停止不会被阻塞；停止后不再重启。
        val backoffMs = minOf(
            WATCHDOG_BACKOFF_BASE_MS shl (attempts - 1),
            WATCHDOG_BACKOFF_MAX_MS
        )
        delay(backoffMs)

        if (!repository.shouldVpnBeRunning()) {
            stopServiceShell()
            return
        }
        startVpn()
    }

    private suspend fun giveUpWatchdogRecovery(reason: String) {
        Log.w("LumineVpn", "Watchdog recovery giving up: $reason")
        suppressAutoRestart = true
        repository.setVpnShouldRun(false)
        VpnRuntimeState.setActive(false)
        VpnRuntimeState.setStatus("error", reason)
        stopServiceShell()
    }

    private fun performCoreShutdownIfNeeded() {
        val shouldStopCore = synchronized(transitionLock) {
            if (coreStopIssued) {
                return@synchronized false
            }
            coreStopIssued = true
            coreOwnsTunFd || coreStarted
        }

        if (shouldStopCore) {
            runCatching { Mobile.stopLumine() }
        } else {
            closePendingTunFd()
        }

        publishPendingLogs()
        coreStarted = false
        coreOwnsTunFd = false
        coreTunFd = null
    }

    private suspend fun stopServiceShell() {
        VpnRuntimeState.setActive(false)
        withContext(Dispatchers.Main) {
            runCatching {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                    stopForeground(STOP_FOREGROUND_REMOVE)
                } else {
                    @Suppress("DEPRECATION")
                    stopForeground(true)
                }
            }
            stopSelf()
            if (VpnRuntimeState.status.value.phase != "error") {
                VpnRuntimeState.setStatus("idle", "点此启动服务")
            }
        }
    }

    @Suppress("DEPRECATION")
    private fun buildNotification(contentText: String): Notification {
        createNotificationChannel()

        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }

        return builder
            .setContentTitle("Lumine")
            .setContentText(contentText)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(contentText: String) {
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, buildNotification(contentText))
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return
        }

        val manager = getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            NOTIFICATION_CHANNEL_ID,
            "Lumine VPN",
            NotificationManager.IMPORTANCE_LOW
        )
        manager.createNotificationChannel(channel)
    }

    companion object {
        const val ACTION_STOP = "STOP"
        const val EXTRA_CONFIG_NAME = "CONFIG_NAME"
        const val EXTRA_REQUEST_VPN_PERMISSION = "REQUEST_VPN_PERMISSION"
        private const val NOTIFICATION_CHANNEL_ID = "lumine_vpn"
        private const val NOTIFICATION_ID = 1001
        private const val WATCHDOG_INTERVAL_MS = 5_000L
        private const val WATCHDOG_RECOVERY_COOLDOWN_MS = 15_000L
        private const val WATCHDOG_BACKOFF_BASE_MS = 5_000L
        private const val WATCHDOG_BACKOFF_MAX_MS = 5 * 60_000L
        private const val WATCHDOG_MAX_RECOVERY_ATTEMPTS = 6
        private const val WATCHDOG_STABLE_RESET_MS = 5 * 60_000L

        @Volatile
        var isServiceRunning = false
    }
}
