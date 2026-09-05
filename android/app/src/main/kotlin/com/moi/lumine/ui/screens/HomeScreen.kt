package com.moi.lumine.ui.screens

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Assignment
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Security
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import org.json.JSONObject

@Composable
fun HomeScreen(
    navController: NavController,
    viewModel: ConfigViewModel,
    onStart: () -> Unit,
    onStop: () -> Unit
) {
    val isConnected by viewModel.isVpnActive.collectAsState()
    val selectedConfig by viewModel.selectedConfigDisplayName.collectAsState()
    val vpnStatus by viewModel.vpnStatus.collectAsState()

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 16.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item {
            Column(modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)) {
                Text(
                    text = "Lumine",
                    style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "本地代理 · 轻量 VPN 客户端",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }

        item {
            StatusCard(
                isConnected = isConnected,
                statusMessage = vpnStatus.message,
                isBusy = vpnStatus.phase == "authorizing" || vpnStatus.phase == "starting" || vpnStatus.phase == "stopping"
            ) {
                if (vpnStatus.phase == "authorizing" || vpnStatus.phase == "starting" || vpnStatus.phase == "stopping") {
                    return@StatusCard
                }
                if (isConnected) onStop() else onStart()
            }
        }

        item {
            SessionStatsLine(active = isConnected)
        }

        item { SectionLabel("配置") }
        item {
            MenuCard(
                title = "配置订阅",
                subtitle = "当前使用：$selectedConfig",
                icon = Icons.Default.Description,
                onClick = { navController.navigate(Screen.Subscriptions.route) }
            )
        }

        item { SectionLabel("更多") }
        item {
            MenuGroup {
                MenuRow(Icons.Default.Tune, "规则") { navController.navigate(Screen.Rules.route) }
                MenuDivider()
                MenuRow(Icons.AutoMirrored.Filled.Assignment, "日志") { navController.navigate(Screen.Logs.route) }
                MenuDivider()
                MenuRow(Icons.Default.Security, "保活设置") { navController.navigate(Screen.KeepAlive.route) }
                MenuDivider()
                MenuRow(Icons.Default.Settings, "设置") { navController.navigate(Screen.Settings.route) }
                MenuDivider()
                MenuRow(Icons.Default.Info, "关于") { navController.navigate(Screen.About.route) }
            }
        }
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.labelLarge,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(top = 8.dp, bottom = 2.dp)
    )
}

@Composable
private fun SessionStatsLine(active: Boolean) {
    var line by remember { mutableStateOf("") }
    LaunchedEffect(active) {
        while (active && isActive) {
            runCatching {
                val json = JSONObject(mobile.Mobile.getStats())
                val down = json.optLong("down", 0L)
                val up = json.optLong("up", 0L)
                val blocked = json.optLong("blocked", 0L)
                val tcp = json.optLong("tcp_conns", 0L)
                val udp = json.optLong("udp_conns", 0L)
                line = "↓ ${formatBytes(homeBytes = down)}  ↑ ${formatBytes(homeBytes = up)}  " +
                    "阻断 $blocked  TCP $tcp / UDP $udp"
            }
            delay(2000L)
        }
        if (!active) line = ""
    }
    if (line.isBlank()) return
    Text(
        text = line,
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp)
    )
}

private fun formatBytes(homeBytes: Long): String {
    if (homeBytes < 0) return "-"
    if (homeBytes < 1024L) return "$homeBytes B"
    if (homeBytes < 1024L * 1024L) return String.format("%.1f KB", homeBytes / 1024f)
    return String.format("%.2f MB", homeBytes / (1024f * 1024f))
}

@Composable
fun StatusCard(isConnected: Boolean, statusMessage: String, isBusy: Boolean, onClick: () -> Unit) {
    val scheme = MaterialTheme.colorScheme
    val isActive = isConnected || isBusy

    val containerColor by animateColorAsState(
        targetValue = if (isActive) scheme.primaryContainer else scheme.surfaceContainerHigh,
        animationSpec = tween(durationMillis = 320),
        label = "status_container"
    )
    val contentColor by animateColorAsState(
        targetValue = if (isActive) scheme.onPrimaryContainer else scheme.onSurface,
        animationSpec = tween(durationMillis = 220),
        label = "status_content"
    )

    val title = when {
        isBusy && isConnected -> "正在断开代理"
        isBusy -> "正在连接代理"
        isConnected -> "代理已连接"
        else -> "代理未连接"
    }
    val summary = when {
        statusMessage.isNotBlank() -> statusMessage
        isConnected -> "流量经本地隧道转发，保持后台即可继续代理。"
        else -> "开启后流量将经 Lumine 隧道转发。"
    }

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = !isBusy, onClick = onClick),
        shape = MaterialTheme.shapes.extraLarge,
        colors = CardDefaults.cardColors(
            containerColor = containerColor,
            contentColor = contentColor
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Surface(
                    modifier = Modifier.size(52.dp),
                    shape = CircleShape,
                    color = contentColor.copy(alpha = 0.12f)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            imageVector = Icons.Default.PowerSettingsNew,
                            contentDescription = null,
                            modifier = Modifier.size(26.dp),
                            tint = contentColor
                        )
                    }
                }
                Spacer(modifier = Modifier.width(16.dp))
                Column(modifier = Modifier.weight(1f)) {
                    AnimatedContent(
                        targetState = title,
                        transitionSpec = { statusContentTransform() },
                        label = "status_title"
                    ) { text ->
                        Text(
                            text = text,
                            style = MaterialTheme.typography.titleLarge,
                            color = contentColor,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    AnimatedContent(
                        targetState = summary,
                        transitionSpec = { statusContentTransform() },
                        label = "status_summary"
                    ) { text ->
                        Text(
                            text = text,
                            style = MaterialTheme.typography.bodyMedium,
                            color = contentColor.copy(alpha = 0.78f),
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
                Spacer(modifier = Modifier.width(12.dp))
                Switch(
                    checked = isConnected,
                    onCheckedChange = null,
                    colors = SwitchDefaults.colors(
                        checkedThumbColor = scheme.onPrimaryContainer,
                        checkedTrackColor = scheme.primary,
                        uncheckedThumbColor = scheme.outline,
                        uncheckedTrackColor = scheme.surfaceContainerHighest,
                        uncheckedBorderColor = scheme.outline
                    )
                )
            }
            if (isBusy) {
                LinearProgressIndicator(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(4.dp),
                    color = scheme.primary
                )
            }
        }
    }
}

@Composable
fun MenuCard(title: String, subtitle: String, icon: ImageVector, onClick: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        shape = MaterialTheme.shapes.extraLarge,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
            contentColor = MaterialTheme.colorScheme.onSurface
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Surface(
                modifier = Modifier.size(48.dp),
                shape = CircleShape,
                color = MaterialTheme.colorScheme.secondaryContainer
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        imageVector = icon,
                        contentDescription = null,
                        modifier = Modifier.size(24.dp),
                        tint = MaterialTheme.colorScheme.onSecondaryContainer
                    )
                }
            }
            Spacer(modifier = Modifier.width(16.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Icon(
                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun MenuGroup(content: @Composable () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.extraLarge,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
            contentColor = MaterialTheme.colorScheme.onSurface
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Column {
            content()
        }
    }
}

@Composable
private fun MenuDivider() {
    HorizontalDivider(
        modifier = Modifier.padding(start = 72.dp),
        color = MaterialTheme.colorScheme.outlineVariant
    )
}

@Composable
private fun MenuRow(icon: ImageVector, title: String, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() }
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Surface(
            modifier = Modifier.size(40.dp),
            shape = CircleShape,
            color = MaterialTheme.colorScheme.secondaryContainer
        ) {
            Box(contentAlignment = Alignment.Center) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    modifier = Modifier.size(20.dp),
                    tint = MaterialTheme.colorScheme.onSecondaryContainer
                )
            }
        }
        Spacer(modifier = Modifier.width(16.dp))
        Text(
            text = title,
            style = MaterialTheme.typography.bodyLarge,
            modifier = Modifier.weight(1f),
            color = MaterialTheme.colorScheme.onSurface
        )
        Icon(
            imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

private fun statusContentTransform(): ContentTransform {
    val duration = 260
    return (fadeIn(animationSpec = tween(durationMillis = duration)) +
        slideInVertically(animationSpec = tween(durationMillis = duration)) { it / 3 }) togetherWith
        (fadeOut(animationSpec = tween(durationMillis = duration)) +
            slideOutVertically(animationSpec = tween(durationMillis = duration)) { -it / 4 })
}

