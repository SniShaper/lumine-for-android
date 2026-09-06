package com.moi.lumine

import android.os.SystemClock
import java.util.ArrayDeque

object ToggleCooldown {

    private const val MAX_ATTEMPTS = 5
    private const val WINDOW_MS = 2_000L
    private const val COOLDOWN_MS = 3_000L

    private val attempts = ArrayDeque<Long>()
    private var cooldownUntil = 0L

    enum class Result {
        ALLOWED,
        COOLDOWN_ACTIVE,
        COOLDOWN_STARTED
    }

    @Synchronized
    fun acquire(): Result {
        val now = SystemClock.elapsedRealtime()
        if (now < cooldownUntil) {
            return Result.COOLDOWN_ACTIVE
        }
        while (attempts.isNotEmpty() && now - attempts.first() > WINDOW_MS) {
            attempts.removeFirst()
        }
        attempts.addLast(now)
        if (attempts.size >= MAX_ATTEMPTS) {
            attempts.clear()
            cooldownUntil = now + COOLDOWN_MS
            return Result.COOLDOWN_STARTED
        }
        return Result.ALLOWED
    }

    @Synchronized
    fun remainingMs(): Long {
        val remain = cooldownUntil - SystemClock.elapsedRealtime()
        return if (remain > 0L) remain else 0L
    }
}
