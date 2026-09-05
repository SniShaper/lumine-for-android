package com.moi.lumine

import android.app.Application
import android.util.Log
import com.moi.lumine.keepalive.KeepAlive
import com.moi.lumine.network.NetworkMonitor
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

class LumineApp : Application() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun onCreate() {
        super.onCreate()

        // 开启过代理才保活：调度闹钟/JobScheduler/WorkManager
        if (KeepAlive.shouldRun(this)) {
            KeepAlive.scheduleAll(this)
        }

        // 网络环境监控（IPv6/NAT64 检测 + 引擎缓存刷新），进程级单例
        NetworkMonitor.start(this)

        startWatchdog()
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
