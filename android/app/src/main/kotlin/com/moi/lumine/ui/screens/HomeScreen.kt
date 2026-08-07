package com.moi.lumine.ui.screens

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Assignment
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen

private val HomePrimaryCardHeight = 100.dp

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
    val context = LocalContext.current

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item {
            Column(modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)) {
                Text(
                    text = "Lumine",
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.Bold
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
        item { MenuRow(Icons.Default.Tune, "规则") { navController.navigate(Screen.Rules.route) } }
        item { MenuRow(Icons.AutoMirrored.Filled.Assignment, "日志") { navController.navigate(Screen.Logs.route) } }
        item { MenuRow(Icons.Default.Security, "保活设置") { navController.navigate(Screen.KeepAlive.route) } }
        item { MenuRow(Icons.Default.Settings, "设置") { navController.navigate(Screen.Settings.route) } }
        item { MenuRow(Icons.Default.Info, "关于") { openProjectPage(context) } }
    }
}

@Composable
private fun SectionLabel(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.titleSmall,
        color = MaterialTheme.colorScheme.primary,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.padding(top = 12.dp, bottom = 2.dp)
    )
}

@Composable
fun StatusCard(isConnected: Boolean, statusMessage: String, isBusy: Boolean, onClick: () -> Unit) {
    val summaryText = when {
        statusMessage.isNotBlank() && (isConnected || isBusy) -> statusMessage
        isConnected -> "服务运行中"
        else -> "点此启动服务"
    }
    val detailText = statusMessage.takeUnless {
        it.isBlank() || it == summaryText || isConnected || isBusy
    }
    val isActive = isConnected || isBusy

    val startColor by animateColorAsState(
        targetValue = if (isActive) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant,
        animationSpec = tween(durationMillis = 320),
        label = "status_gradient_start"
    )
    val endColor by animateColorAsState(
        targetValue = if (isActive) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.surfaceVariant,
        animationSpec = tween(durationMillis = 320),
        label = "status_gradient_end"
    )
    val contentColor by animateColorAsState(
        targetValue = if (isActive) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
        animationSpec = tween(durationMillis = 220),
        label = "status_content"
    )
    val summaryColor by animateColorAsState(
        targetValue = if (isActive) Color.White.copy(alpha = 0.85f) else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
        animationSpec = tween(durationMillis = 320),
        label = "status_summary"
    )
    val iconScale by animateFloatAsState(
        targetValue = if (isActive) 1.08f else 1f,
        animationSpec = tween(durationMillis = 320),
        label = "status_icon_scale"
    )
    val cardElevation by animateDpAsState(
        targetValue = if (isActive) 8.dp else 2.dp,
        animationSpec = tween(durationMillis = 320),
        label = "status_elevation"
    )

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .height(HomePrimaryCardHeight)
            .clickable { onClick() },
        colors = CardDefaults.cardColors(containerColor = Color.Transparent),
        elevation = CardDefaults.cardElevation(defaultElevation = cardElevation)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(Brush.linearGradient(listOf(startColor, endColor)))
        ) {
            Row(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(16.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Surface(
                    modifier = Modifier.size(56.dp),
                    shape = CircleShape,
                    color = contentColor.copy(alpha = 0.16f)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            imageVector = if (isConnected) Icons.Default.CheckCircle else Icons.Default.Cancel,
                            contentDescription = null,
                            modifier = Modifier
                                .size(34.dp)
                                .graphicsLayer {
                                    scaleX = iconScale
                                    scaleY = iconScale
                                },
                            tint = contentColor
                        )
                    }
                }
                Spacer(modifier = Modifier.width(16.dp))
                Column(modifier = Modifier.weight(1f)) {
                    AnimatedContent(
                        targetState = if (isConnected) "已启动" else "已停止",
                        transitionSpec = { statusContentTransform() },
                        label = "status_title"
                    ) { title ->
                        Text(
                            text = title,
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Bold,
                            color = contentColor,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    AnimatedContent(
                        targetState = summaryText,
                        transitionSpec = { statusContentTransform() },
                        label = "status_summary_text"
                    ) { text ->
                        Text(
                            text = text,
                            style = MaterialTheme.typography.bodyMedium,
                            color = summaryColor,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    if (detailText != null) {
                        Text(
                            text = detailText,
                            style = MaterialTheme.typography.bodySmall,
                            color = summaryColor,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
                Spacer(modifier = Modifier.width(12.dp))
                if (isBusy) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(22.dp),
                        strokeWidth = 2.dp,
                        color = contentColor
                    )
                }
                StatusPill(isConnected = isConnected, isBusy = isBusy, tint = contentColor)
            }
        }
    }
}

@Composable
private fun StatusPill(isConnected: Boolean, isBusy: Boolean, tint: Color) {
    val label = when {
        isBusy -> "处理中"
        isConnected -> "已连接"
        else -> "离线"
    }
    Surface(
        shape = CircleShape,
        color = tint.copy(alpha = 0.18f)
    ) {
        Text(
            text = label,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            style = MaterialTheme.typography.labelMedium,
            color = tint,
            fontWeight = FontWeight.SemiBold
        )
    }
}

@Composable
fun MenuCard(title: String, subtitle: String, icon: ImageVector, onClick: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .height(HomePrimaryCardHeight)
            .clickable { onClick() },
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerLow
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconContainer(icon)
            Spacer(modifier = Modifier.width(16.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                Spacer(modifier = Modifier.height(2.dp))
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
private fun MenuRow(icon: ImageVector, title: String, onClick: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerLow
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconContainer(icon)
            Spacer(modifier = Modifier.width(16.dp))
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                modifier = Modifier.weight(1f)
            )
            Icon(
                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun IconContainer(icon: ImageVector) {
    Surface(
        modifier = Modifier.size(44.dp),
        shape = CircleShape,
        color = MaterialTheme.colorScheme.secondaryContainer
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSecondaryContainer
            )
        }
    }
}

private fun statusContentTransform(): ContentTransform {
    val duration = 260
    return (fadeIn(animationSpec = tween(durationMillis = duration)) +
        slideInVertically(animationSpec = tween(durationMillis = duration)) { it / 3 }) togetherWith
        (fadeOut(animationSpec = tween(durationMillis = duration)) +
            slideOutVertically(animationSpec = tween(durationMillis = duration)) { -it / 4 })
}

private fun openProjectPage(context: Context) {
    val uri = Uri.parse("https://github.com/coolapijust/lumine-for-android")
    val baseIntent = Intent(Intent.ACTION_VIEW, uri).apply {
        addCategory(Intent.CATEGORY_BROWSABLE)
        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
    val packageManager = context.packageManager
    val candidates = packageManager.queryIntentActivities(baseIntent, 0)
        .map { it.activityInfo.packageName }
        .distinct()
        .filter { it != context.packageName }

    val intent = Intent(baseIntent)
    val resolved = baseIntent.resolveActivity(packageManager)?.packageName
    val preferredPackage = when {
        resolved != null && resolved != context.packageName -> resolved
        candidates.isNotEmpty() -> candidates.first()
        else -> null
    }
    if (preferredPackage != null) {
        intent.setPackage(preferredPackage)
    }
    runCatching { context.startActivity(intent) }
}
