package com.moi.lumine.ui.screens

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
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen
import com.moi.lumine.ui.theme.GreenConnect

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
    val uriHandler = LocalUriHandler.current

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

        // Menu Items
        item { MenuItem(Icons.Default.Tune, "规则", "查看并编辑当前配置规则") { navController.navigate(Screen.Rules.route) } }
        item { MenuItem(Icons.AutoMirrored.Filled.Assignment, "日志", "查看实时核心输出") { navController.navigate(Screen.Logs.route) } }
        item { MenuItem(Icons.Default.Settings, "设置", "修改全局参数与 DoH") { navController.navigate(Screen.Settings.route) } }
        item {
            MenuItem(Icons.Default.Info, "关于", "打开项目主页") {
                uriHandler.openUri("http://github.com/coolapijust/lumine-for-android")
            }
        }
    }
}

@Composable
fun StatusCard(isConnected: Boolean, statusMessage: String, isBusy: Boolean, onClick: () -> Unit) {
    val summaryText = if (isConnected) "服务运行中" else "点此启动服务"
    val detailText = statusMessage.takeUnless { it.isBlank() || it == summaryText }

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .height(100.dp)
            .clickable { onClick() },
        colors = CardDefaults.cardColors(
            containerColor = if (isConnected) GreenConnect.copy(alpha = 0.8f) else MaterialTheme.colorScheme.surfaceVariant
        )
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
                modifier = Modifier.size(48.dp),
                tint = if (isConnected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.width(16.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = if (isConnected) "已启动" else "已停止",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                    color = if (isConnected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = summaryText,
                    style = MaterialTheme.typography.bodyMedium,
                    color = if (isConnected) MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.8f) else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (detailText != null) {
                    Text(
                        text = detailText,
                        style = MaterialTheme.typography.bodySmall,
                        color = if (isConnected) MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.75f) else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
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
                    color = if (isConnected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.primary
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
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
    ) {
        ListItem(
            headlineContent = { Text(title, fontWeight = FontWeight.Bold) },
            supportingContent = { Text(subtitle) },
            leadingContent = { Icon(icon, contentDescription = null, modifier = Modifier.size(32.dp)) }
        )
    }
}

@Composable
fun MenuItem(icon: ImageVector, title: String, subtitle: String, onClick: () -> Unit) {
    ListItem(
        headlineContent = { Text(title) },
        supportingContent = { Text(subtitle) },
        leadingContent = { Icon(icon, contentDescription = null) },
        modifier = Modifier.clickable { onClick() }
    )
}
