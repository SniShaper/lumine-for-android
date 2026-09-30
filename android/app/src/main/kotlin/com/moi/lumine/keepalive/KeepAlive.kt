package com.moi.lumine.keepalive

import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.core.content.ContextCompat
import com.moi.lumine.LumineVpnService
import com.moi.lumine.repository.ConfigRepository

object KeepAlive {

    @Volatile
    private var repo: ConfigRepository? = null

    private fun repository(context: Context): ConfigRepository {
        val cached = repo
        if (cached != null) return cached
        synchronized(this) {
            val current = repo
            if (current != null) return current
            return ConfigRepository(context.applicationContext).also { repo = it }
        }
    }

    fun shouldRun(context: Context): Boolean =
        repository(context).shouldVpnBeRunning()

    fun tryRestart(context: Context) {
        if (!shouldRun(context)) return
        if (LumineVpnService.isServiceRunning) return
        try {
            ContextCompat.startForegroundService(
                context.applicationContext,
                Intent(context.applicationContext, LumineVpnService::class.java)
            )
            Log.d("KeepAlive", "已拉起 Lumine 代理服务")
        } catch (e: Exception) {
            Log.w("KeepAlive", "拉起服务失败: ${e.message}")
        }
    }

    fun scheduleAll(context: Context) {
        ServiceRestartReceiver.scheduleAlarm(context)
        KeepAliveJobService.schedule(context)
        ServiceWatchdogWorker.schedule(context)
    }

    fun cancelAll(context: Context) {
        ServiceRestartReceiver.cancelAlarm(context)
        KeepAliveJobService.cancel(context)
        ServiceWatchdogWorker.cancel(context)
    }
}
