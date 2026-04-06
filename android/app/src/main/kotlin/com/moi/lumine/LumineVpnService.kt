package com.moi.lumine

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import java.io.File
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mobile.Mobile // This will be available after gomobile bind

class LumineVpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null
    private var coreTunFd: Int? = null
    private var configName: String = "config" // Default config name
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    @Volatile private var isStarting = false
    @Volatile private var isStopping = false

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val action = intent?.action
        if (action == "STOP") {
            VpnRuntimeState.setStatus("stopping", "正在停止代理")
            stopVpn()
            return START_NOT_STICKY
        }

        configName = intent?.getStringExtra("CONFIG_NAME") ?: "config"
        startVpn()
        return START_STICKY
    }

    private fun startVpn() {
        if (isStarting || isStopping || vpnInterface != null) {
            return
        }

        try {
            isStarting = true
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
                        Mobile.setWorkingDir(filesDir.absolutePath)
                        val error = Mobile.startLumine(fd.toLong(), configName)
                        if (error.isNotEmpty()) {
                            runCatching { ParcelFileDescriptor.adoptFd(fd).close() }
                            coreTunFd = null
                            Log.e("LumineVpn", "Go core failed: $error")
                            updateNotification("启动失败: $error")
                            VpnRuntimeState.setStatus("error", "启动失败: $error")
                            stopVpn()
                        } else {
                            Log.i("LumineVpn", "Lumine started successfully")
                            VpnRuntimeState.setStatus("running", "代理运行中")
                            updateNotification("代理运行中")
                        }
                    } catch (e: Exception) {
                        Log.e("LumineVpn", "Failed to initialize Go core", e)
                        VpnRuntimeState.setStatus("error", "核心初始化失败")
                        stopVpn()
                    } finally {
                        isStarting = false
                    }
                }
            } else {
                isStarting = false
                VpnRuntimeState.setStatus("error", "VPN 建立失败")
            }
        } catch (e: Exception) {
            Log.e("LumineVpn", "Failed to start VPN", e)
            isStarting = false
            VpnRuntimeState.setStatus("error", "启动 VPN 失败")
            stopVpn()
        }
    }

    private fun stopVpn() {
        if (isStopping) {
            return
        }
        isStopping = true
        serviceScope.launch {
            try {
                runCatching { Mobile.stopLumine() }
                coreTunFd = null

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
        serviceScope.cancel()
        runCatching { Mobile.stopLumine() }
        coreTunFd = null
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
        private const val NOTIFICATION_CHANNEL_ID = "lumine_vpn"
        private const val NOTIFICATION_ID = 1001
    }
}
