package com.moi.lumine.keepalive

import android.accessibilityservice.AccessibilityService
import android.util.Log
import android.view.accessibility.AccessibilityEvent

/**
 * 无障碍保活服务
 *
 * 原理：用户划掉app时，系统会杀主进程，但无障碍服务是系统绑定的，
 * 系统会立刻重新创建该服务进程，触发 onServiceConnected，
 * 我们顺势拉起前台代理服务。
 *
 * 使用方法：
 * 1. 手机设置 → 无障碍/辅助功能 → 找到 "Lumine" → 开启
 * 2. 首次开启时系统会弹窗确认，点击允许即可
 */
class KeepAliveAccessibilityService : AccessibilityService() {

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        // 不需要处理任何无障碍事件
    }

    override fun onInterrupt() {}

    override fun onServiceConnected() {
        super.onServiceConnected()
        Log.d("KeepAlive", "无障碍服务已连接，检查代理服务")
        KeepAlive.tryRestart(this)
    }
}
