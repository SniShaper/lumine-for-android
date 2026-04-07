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
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Assignment
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.font.FontWeight
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
            Text(
                text = "Lumine for Android",
                style = MaterialTheme.typography.headlineSmall,
                modifier = Modifier.padding(bottom = 8.dp)
            )
        }

        // VPN Status Card
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

        // Profile Card
        item {
            MenuCard(
                title = "配置",
                subtitle = "当前使用：$selectedConfig",
                icon = Icons.Default.Description,
                onClick = { navController.navigate(Screen.Subscriptions.route) }
            )
        }

        item { MenuItem(Icons.Default.Tune, "规则") { navController.navigate(Screen.Rules.route) } }
        item { MenuItem(Icons.AutoMirrored.Filled.Assignment, "日志") { navController.navigate(Screen.Logs.route) } }
        item { MenuItem(Icons.Default.Settings, "设置") { navController.navigate(Screen.Settings.route) } }
        item {
            MenuItem(Icons.Default.Info, "关于") {
                openProjectPage(context)
            }
        }
    }
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
    val containerColor by animateColorAsState(
        targetValue = if (isConnected || isBusy) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant,
        animationSpec = tween(durationMillis = 220),
        label = "status_container"
    )
    val contentColor by animateColorAsState(
        targetValue = if (isConnected || isBusy) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
        animationSpec = tween(durationMillis = 220),
        label = "status_content"
    )
    val summaryColor by animateColorAsState(
        targetValue = if (isConnected || isBusy) MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.84f) else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
        animationSpec = tween(durationMillis = 320),
        label = "status_summary"
    )
    val iconScale by animateFloatAsState(
        targetValue = if (isConnected || isBusy) 1.08f else 1f,
        animationSpec = tween(durationMillis = 320),
        label = "status_icon_scale"
    )
    val cardElevation by animateDpAsState(
        targetValue = if (isConnected || isBusy) 8.dp else 2.dp,
        animationSpec = tween(durationMillis = 320),
        label = "status_elevation"
    )

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .height(HomePrimaryCardHeight)
            .clickable { onClick() },
        colors = CardDefaults.cardColors(
            containerColor = containerColor
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = cardElevation)
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(
                imageVector = if (isConnected) Icons.Default.CheckCircle else Icons.Default.Cancel,
                contentDescription = null,
                modifier = Modifier
                    .size(48.dp)
                    .graphicsLayer {
                        scaleX = iconScale
                        scaleY = iconScale
                    },
                tint = contentColor
            )
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
            if (isBusy) {
                Spacer(modifier = Modifier.width(12.dp))
                CircularProgressIndicator(
                    modifier = Modifier.size(24.dp),
                    strokeWidth = 2.dp,
                    color = contentColor
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
            .height(HomePrimaryCardHeight)
            .clickable { onClick() },
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        ListItem(
            headlineContent = { Text(title, fontWeight = FontWeight.Bold) },
            supportingContent = {
                Text(
                    text = subtitle,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            },
            leadingContent = { Icon(icon, contentDescription = null, modifier = Modifier.size(32.dp)) },
            colors = ListItemDefaults.colors(
                containerColor = MaterialTheme.colorScheme.surfaceVariant
            ),
            modifier = Modifier.fillMaxSize()
        )
    }
}

@Composable
fun MenuItem(icon: ImageVector, title: String, onClick: () -> Unit) {
    ListItem(
        headlineContent = { Text(title) },
        leadingContent = { Icon(icon, contentDescription = null) },
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() }
    )
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
