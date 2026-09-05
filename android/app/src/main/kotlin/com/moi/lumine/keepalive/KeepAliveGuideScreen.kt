package com.moi.lumine.keepalive

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController

data class PermissionItem(
    val title: String,
    val description: String,
    val isGranted: Boolean,
    val actionLabel: String,
    val onClick: () -> Unit
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun KeepAliveGuideScreen(
    navController: NavController
) {
    val context = LocalContext.current
    val manufacturer = Build.MANUFACTURER.lowercase()

    val items = listOf(
        PermissionItem(
            title = "开启无障碍服务（核心）",
            description = "手机设置 → 无障碍/辅助功能 → Lumine → 开启。\n划掉app后系统会自动重启此服务，1~3秒内恢复代理",
            isGranted = isAccessibilityServiceEnabled(context),
            actionLabel = "去设置",
            onClick = { openAccessibilitySettings(context) }
        ),
        PermissionItem(
            title = "允许自启动",
            description = getAutoStartDescription(manufacturer),
            isGranted = false,
            actionLabel = "去设置",
            onClick = { openAutoStartSettings(context, manufacturer) }
        ),
        PermissionItem(
            title = "关闭电池优化",
            description = "允许应用在后台不受电池优化限制，持续运行",
            isGranted = isBatteryOptimizationDisabled(context),
            actionLabel = "去设置",
            onClick = { openBatteryOptimizationSettings(context) }
        ),
        PermissionItem(
            title = "允许后台活动",
            description = getBackgroundActivityDescription(manufacturer),
            isGranted = false,
            actionLabel = "去设置",
            onClick = { openAppInfoSettings(context) }
        ),
        PermissionItem(
            title = "锁定最近任务",
            description = "在多任务界面长按应用卡片，点击锁定图标，防止被清理",
            isGranted = false,
            actionLabel = "知道了",
            onClick = {}
        ),
        PermissionItem(
            title = "通知权限",
            description = "确保通知权限已开启，服务通知是保活的关键",
            isGranted = false,
            actionLabel = "去设置",
            onClick = { openNotificationSettings(context) }
        )
    )

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("保活设置") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = "Back"
                        )
                    }
                }
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp)
        ) {
            // Warning card
            Card(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 16.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.errorContainer,
                    contentColor = MaterialTheme.colorScheme.onErrorContainer
                ),
                shape = MaterialTheme.shapes.extraLarge
            ) {
                Row(
                    modifier = Modifier.padding(16.dp),
                    verticalAlignment = Alignment.Top
                ) {
                    Icon(
                        imageVector = Icons.Filled.Warning,
                        contentDescription = null,
                        modifier = Modifier.size(24.dp),
                        tint = MaterialTheme.colorScheme.onErrorContainer
                    )
                    Spacer(modifier = Modifier.width(12.dp))
                    Column {
                        Text(
                            text = "为什么需要这些设置？",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onErrorContainer
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            text = "ColorOS、MIUI等国产系统会在用户划掉应用时强制杀死进程，导致代理中断。\n\n" +
                                    "开启以下权限后，应用才能在后台持续运行，保持代理连接和通知显示。",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onErrorContainer.copy(alpha = 0.9f)
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(20.dp))

            items.forEachIndexed { index, item ->
                PermissionCardWithStep(
                    stepNumber = index + 1,
                    item = item
                )
                if (index < items.lastIndex) {
                    Spacer(modifier = Modifier.height(12.dp))
                }
            }

            Spacer(modifier = Modifier.height(24.dp))
        }
    }
}

@Composable
private fun PermissionCardWithStep(
    stepNumber: Int,
    item: PermissionItem
) {
    val scheme = MaterialTheme.colorScheme
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = scheme.surfaceContainerLow,
            contentColor = scheme.onSurface
        ),
        shape = MaterialTheme.shapes.extraLarge
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.Top
        ) {
            // Step indicator
            Box(
                modifier = Modifier
                    .size(32.dp)
                    .clip(CircleShape)
                    .background(
                        if (item.isGranted) scheme.primary
                        else scheme.secondaryContainer
                    ),
                contentAlignment = Alignment.Center
            ) {
                if (item.isGranted) {
                    Icon(
                        imageVector = Icons.Filled.Check,
                        contentDescription = null,
                        modifier = Modifier.size(18.dp),
                        tint = scheme.onPrimary
                    )
                } else {
                    Text(
                        text = stepNumber.toString(),
                        style = MaterialTheme.typography.labelMedium,
                        color = scheme.onSecondaryContainer
                    )
                }
            }

            Spacer(modifier = Modifier.width(12.dp))

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = scheme.onSurface
                )
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = item.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = scheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedButton(
                    onClick = item.onClick,
                    shape = MaterialTheme.shapes.small
                ) {
                    Text(
                        text = item.actionLabel,
                        style = MaterialTheme.typography.labelLarge
                    )
                }
            }

            // Status icon
            if (item.isGranted) {
                Icon(
                    imageVector = Icons.Filled.CheckCircle,
                    contentDescription = "已设置",
                    tint = scheme.primary,
                    modifier = Modifier.size(20.dp)
                )
            }
        }
    }
}

private fun isAutoStartEnabled(context: Context): Boolean {
    return try {
        val intent = Intent().apply {
            component = ComponentName(
                "com.coloros.safecenter",
                "com.coloros.safecenter.permission.startup.StartupAppListActivity"
            )
        }
        context.startActivity(intent)
        true
    } catch (_: Exception) {
        try {
            val intent = Intent().apply {
                component = ComponentName(
                    "com.miui.securitycenter",
                    "com.miui.permcenter.autostart.AutoStartManagementActivity"
                )
            }
            context.startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }
}

private fun getAutoStartDescription(manufacturer: String): String {
    return when {
        manufacturer.contains("oppo") || manufacturer.contains("coloros") ->
            "设置 → 电池 → 自启动管理 → 开启 Lumine"
        manufacturer.contains("xiaomi") || manufacturer.contains("miui") ->
            "设置 → 应用设置 → 自启动管理 → 开启 Lumine"
        manufacturer.contains("huawei") || manufacturer.contains("emui") ->
            "设置 → 电池 → 启动管理 → 手动管理 → Lumine → 全部开启"
        manufacturer.contains("vivo") || manufacturer.contains("funtouch") ->
            "i管家 → 应用管理 → 权限管理 → 自启动 → 开启 Lumine"
        else -> "确保应用可以在后台自动启动"
    }
}

private fun openAutoStartSettings(context: Context, manufacturer: String) {
    try {
        val intent = when {
            manufacturer.contains("oppo") || manufacturer.contains("coloros") -> {
                Intent().apply {
                    component = ComponentName(
                        "com.coloros.safecenter",
                        "com.coloros.safecenter.permission.startup.StartupAppListActivity"
                    )
                }
            }
            manufacturer.contains("xiaomi") || manufacturer.contains("miui") -> {
                Intent().apply {
                    component = ComponentName(
                        "com.miui.securitycenter",
                        "com.miui.permcenter.autostart.AutoStartManagementActivity"
                    )
                }
            }
            manufacturer.contains("huawei") || manufacturer.contains("emui") -> {
                Intent().apply {
                    component = ComponentName(
                        "com.huawei.systemmanager",
                        "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity"
                    )
                }
            }
            manufacturer.contains("vivo") || manufacturer.contains("funtouch") -> {
                Intent().apply {
                    component = ComponentName(
                        "com.iqoo.secure",
                        "com.iqoo.secure.ui.phoneoptimize.AddWhiteListActivity"
                    )
                }
            }
            else -> {
                Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                    data = Uri.parse("package:${context.packageName}")
                }
            }
        }
        context.startActivity(intent)
    } catch (_: Exception) {
        try {
            val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
            intent.data = Uri.parse("package:${context.packageName}")
            context.startActivity(intent)
        } catch (_: Exception) {}
    }
}

private fun isBatteryOptimizationDisabled(context: Context): Boolean {
    val pm = context.getSystemService(Context.POWER_SERVICE) as PowerManager
    return pm.isIgnoringBatteryOptimizations(context.packageName)
}

private fun openBatteryOptimizationSettings(context: Context) {
    try {
        val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
            data = Uri.parse("package:${context.packageName}")
        }
        context.startActivity(intent)
    } catch (_: Exception) {
        try {
            val intent = Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)
            context.startActivity(intent)
        } catch (_: Exception) {}
    }
}

private fun getBackgroundActivityDescription(manufacturer: String): String {
    return when {
        manufacturer.contains("oppo") || manufacturer.contains("coloros") ->
            "设置 → 应用管理 → Lumine → 耗电管理 → 允许后台活动"
        manufacturer.contains("xiaomi") || manufacturer.contains("miui") ->
            "设置 → 应用设置 → 应用管理 → Lumine → 省电策略 → 无限制"
        manufacturer.contains("huawei") || manufacturer.contains("emui") ->
            "设置 → 电池 → 启动管理 → Lumine → 手动管理 → 后台活动开启"
        manufacturer.contains("vivo") || manufacturer.contains("funtouch") ->
            "i管家 → 应用管理 → 权限管理 → 后台活动 → 开启 Lumine"
        else -> "确保应用可以在后台活动"
    }
}

private fun openAppInfoSettings(context: Context) {
    try {
        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
        intent.data = Uri.parse("package:${context.packageName}")
        context.startActivity(intent)
    } catch (_: Exception) {}
}

private fun openNotificationSettings(context: Context) {
    try {
        val intent = Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).apply {
            putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
        }
        context.startActivity(intent)
    } catch (_: Exception) {
        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
        intent.data = Uri.parse("package:${context.packageName}")
        context.startActivity(intent)
    }
}

private fun isAccessibilityServiceEnabled(context: Context): Boolean {
    val serviceName = "${context.packageName}/com.moi.lumine.keepalive.KeepAliveAccessibilityService"
    val enabledServices = Settings.Secure.getString(
        context.contentResolver,
        Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES
    ) ?: return false
    return enabledServices.split(":").any { it.equals(serviceName, ignoreCase = true) }
}

private fun openAccessibilitySettings(context: Context) {
    try {
        val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
        context.startActivity(intent)
    } catch (_: Exception) {
        try {
            val intent = Intent().apply {
                component = ComponentName(
                    "com.coloros.safecenter",
                    "com.coloros.safecenter无障碍.AccessibilityListActivity"
                )
            }
            context.startActivity(intent)
        } catch (_: Exception) {
            try {
                val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
                intent.data = Uri.parse("package:${context.packageName}")
                context.startActivity(intent)
            } catch (_: Exception) {}
        }
    }
}
