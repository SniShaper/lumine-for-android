package com.moi.lumine.keepalive

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val action = intent.action
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED) return

        // 未开启代理 → 不保活
        if (!KeepAlive.shouldRun(context)) return

        val pendingResult = goAsync()
        // 作用域生命周期与 goAsync 绑定：任务结束（含异常）后 cancel，
        // 避免每次广播遗留一个永不取消的 CoroutineScope。
        val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
        scope.launch {
            try {
                KeepAlive.tryRestart(context)
                KeepAlive.scheduleAll(context)
            } finally {
                pendingResult.finish()
                scope.cancel()
            }
        }
    }
}
