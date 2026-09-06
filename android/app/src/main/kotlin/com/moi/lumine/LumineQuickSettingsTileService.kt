package com.moi.lumine

import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import androidx.core.content.ContextCompat
import com.moi.lumine.repository.ConfigRepository

class LumineQuickSettingsTileService : TileService() {

    private val handler = Handler(Looper.getMainLooper())
    private var lastRenderedState = -1
    private var tileListening = false
    private var lastActionAt = 0L

    private val updater = object : Runnable {
        override fun run() {
            renderState()
            if (tileListening) {
                handler.postDelayed(this, REFRESH_INTERVAL_MS)
            }
        }
    }

    override fun onTileAdded() {
        super.onTileAdded()
        renderState()
    }

    override fun onStartListening() {
        super.onStartListening()
        tileListening = true
        renderState()
        handler.removeCallbacks(updater)
        handler.postDelayed(updater, REFRESH_INTERVAL_MS)
    }

    override fun onStopListening() {
        super.onStopListening()
        tileListening = false
        handler.removeCallbacks(updater)
    }

    override fun onClick() {
        val now = SystemClock.elapsedRealtime()
        if (now - lastActionAt < MIN_ACTION_INTERVAL_MS) {
            return
        }
        lastActionAt = now

        val status = VpnRuntimeState.status.value
        val running = VpnRuntimeState.isVpnActive.value
        val busy = status.phase == "starting" ||
            status.phase == "stopping" ||
            status.phase == "authorizing"
        if (busy) {
            return
        }

        val context = applicationContext
        val repository = ConfigRepository(context)

        if (running) {
            val stop = Intent(context, LumineVpnService::class.java).apply {
                action = LumineVpnService.ACTION_STOP
            }
            runCatching { context.startService(stop) }
            return
        }

        repository.setVpnShouldRun(true)

        val vpnIntent = VpnService.prepare(context)
        if (vpnIntent != null) {
            val authorize = Intent(context, MainActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                putExtra(LumineVpnService.EXTRA_REQUEST_VPN_PERMISSION, true)
            }
            startActivityAndCollapse(authorize)
            return
        }

        val start = Intent(context, LumineVpnService::class.java).apply {
            putExtra(LumineVpnService.EXTRA_CONFIG_NAME, repository.getSelectedConfigName())
        }
        val started = runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                ContextCompat.startForegroundService(context, start)
            } else {
                context.startService(start)
            }
        }
        if (started.isFailure) {
            startActivityAndCollapse(
                Intent(context, MainActivity::class.java).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            )
        }
        renderState()
    }

    private fun renderState() {
        val tile = qsTile ?: return
        val status = VpnRuntimeState.status.value
        val running = VpnRuntimeState.isVpnActive.value
        val target = when {
            running || status.phase == "starting" -> Tile.STATE_ACTIVE
            status.phase == "authorizing" -> Tile.STATE_UNAVAILABLE
            else -> Tile.STATE_INACTIVE
        }
        if (target == lastRenderedState) {
            return
        }
        lastRenderedState = target
        tile.state = target
        tile.label = when (target) {
            Tile.STATE_ACTIVE -> "Lumine 运行中"
            Tile.STATE_UNAVAILABLE -> "Lumine 待授权"
            else -> "Lumine 已停止"
        }
        runCatching { tile.updateTile() }
    }

    companion object {
        private const val REFRESH_INTERVAL_MS = 800L
        private const val MIN_ACTION_INTERVAL_MS = 500L
    }
}
