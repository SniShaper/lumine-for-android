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
import com.moi.lumine.repository.ConfigRepository
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mobile.Mobile // This will be available after gomobile bind

class LumineVpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null
    private var coreTunFd: Int? = null
    private var configName: String = "config" // Default config name
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val repository by lazy { ConfigRepository(applicationContext) }
    private val transitionLock = Any()
    private var logPumpJob: Job? = null
    private var watchdogJob: Job? = null
    @Volatile private var isStarting = false
    @Volatile private var isStopping = false
    @Volatile private var coreStarted = false
    @Volatile private var coreOwnsTunFd = false
    @Volatile private var pendingStopRequested = false
    @Volatile private var coreStopIssued = false
    @Volatile private var lastWatchdogRecoveryAt = 0L

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val action = intent?.action
        if (action == ACTION_STOP) {
            repository.setVpnShouldRun(false)
            VpnRuntimeState.setStatus("stopping", "正在停止代理")
            stopVpn()
            return START_NOT_STICKY
        }

        val requestedConfig = intent?.getStringExtra(EXTRA_CONFIG_NAME)?.takeIf { it.isNotBlank() }
        val shouldRecover = requestedConfig == null && repository.shouldVpnBeRunning()
        val targetConfig = requestedConfig ?: if (shouldRecover) repository.getLastRunningConfigName() else null

        if (targetConfig == null) {
            Log.i("LumineVpn", "Ignoring sticky restart without persisted running state")
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
            builder.addDisallowedApplication(packageName)

            vpnInterface = builder.establish()

            if (vpnInterface != null) {
                val tun = vpnInterface!!
                val fd = tun.detachFd()
                vpnInterface = null
                coreTunFd = fd
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
                        if (error.isNotEmpty()) {
                            coreOwnsTunFd = false
                            closePendingTunFd()
                            Log.e("LumineVpn", "Go core failed: $error")
                            updateNotification("启动失败: $error")
                            VpnRuntimeState.setActive(false)
                            VpnRuntimeState.setStatus("error", "启动失败: $error")
                            repository.setVpnShouldRun(false)
                            stopVpn()
                        } else {
                            coreStarted = true
                            Log.i("LumineVpn", "Lumine started successfully")
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
                pendingStopRequested = false
                VpnRuntimeState.setActive(false)

                val tun = vpnInterface
                vpnInterface = null
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

    override fun onDestroy() {
        stopWatchdog()
        stopLogPump()
        performCoreShutdownIfNeeded()
        serviceScope.cancel()
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

    private fun ensureConfigFile(name: String) {
        val target = File(filesDir, "$name.json")
        if (target.exists()) {
            return
        }

        if (name == "config") {
            assets.open("config_default.json").use { input ->
                target.outputStream().use { output ->
                    input.copyTo(output)
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
        val fd = coreTunFd ?: return
        coreTunFd = null
        runCatching { ParcelFileDescriptor.adoptFd(fd).close() }
    }

    private suspend fun recoverVpnFromWatchdog() {
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

            val tun = vpnInterface
            vpnInterface = null
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

        startVpn()
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
        private const val ACTION_STOP = "STOP"
        private const val EXTRA_CONFIG_NAME = "CONFIG_NAME"
        private const val NOTIFICATION_CHANNEL_ID = "lumine_vpn"
        private const val NOTIFICATION_ID = 1001
        private const val WATCHDOG_INTERVAL_MS = 5_000L
        private const val WATCHDOG_RECOVERY_COOLDOWN_MS = 15_000L
    }
}
