package com.moi.lumine.keepalive

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.SystemClock
import com.moi.lumine.LumineVpnService
import com.moi.lumine.MainActivity

class ServiceRestartReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        if (!KeepAlive.shouldRun(context)) {
            KeepAlive.cancelAll(context)
            consecutiveFailures = 0
            return
        }

        if (LumineVpnService.isServiceRunning) {
            consecutiveFailures = 0
            return
        }

        KeepAlive.tryRestart(context)

        if (LumineVpnService.isServiceRunning) {
            consecutiveFailures = 0
            return
        }

        val delayMs = minOf(
            BASE_DELAY_MS shl consecutiveFailures.coerceAtMost(6),
            MAX_DELAY_MS
        )
        consecutiveFailures += 1
        scheduleAlarm(context, delayMs)
    }

    companion object {
        private const val REQUEST_CODE = 9999
        private const val BASE_DELAY_MS = 10_000L
        private const val MAX_DELAY_MS = 5 * 60_000L

        @Volatile
        private var consecutiveFailures = 0

        fun scheduleAlarm(context: Context, delayMs: Long = BASE_DELAY_MS) {
            try {
                val restartIntent = Intent(context, ServiceRestartReceiver::class.java)
                val pendingIntent = PendingIntent.getBroadcast(
                    context, REQUEST_CODE, restartIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
                val triggerTime = SystemClock.elapsedRealtime() + delayMs

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
                consecutiveFailures = 0
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
