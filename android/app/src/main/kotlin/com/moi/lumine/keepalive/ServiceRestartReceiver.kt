package com.moi.lumine.keepalive

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.SystemClock
import com.moi.lumine.MainActivity

class ServiceRestartReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        // 未开启代理 → 取消全部保活调度
        if (!KeepAlive.shouldRun(context)) {
            KeepAlive.cancelAll(context)
            return
        }

        KeepAlive.tryRestart(context)

        // 设下一次闹钟（链），15 秒后再次检查
        KeepAlive.scheduleAll(context)
    }

    companion object {
        private const val REQUEST_CODE = 9999

        fun scheduleAlarm(context: Context) {
            try {
                val restartIntent = Intent(context, ServiceRestartReceiver::class.java)
                val pendingIntent = PendingIntent.getBroadcast(
                    context, REQUEST_CODE, restartIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
                val triggerTime = SystemClock.elapsedRealtime() + 10_000L

                // showIntent 必须指向 Activity，否则部分厂商不触发
                val showIntent = Intent(context, MainActivity::class.java)
                val showPending = PendingIntent.getActivity(
                    context, REQUEST_CODE + 1, showIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                val alarmInfo = AlarmManager.AlarmClockInfo(triggerTime, showPending)
                alarmManager.setAlarmClock(alarmInfo, pendingIntent)
            } catch (_: Exception) {}
        }

        fun cancelAlarm(context: Context) {
            try {
                val restartIntent = Intent(context, ServiceRestartReceiver::class.java)
                val pendingIntent = PendingIntent.getBroadcast(
                    context, REQUEST_CODE, restartIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
                alarmManager.cancel(pendingIntent)
            } catch (_: Exception) {}
        }
    }
}
