package com.moi.lumine.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RoutingScreen(navController: NavController, viewModel: ConfigViewModel) {
    val config by viewModel.currentConfig.collectAsState()
    val isConnected by viewModel.isVpnActive.collectAsState()
    var mode by remember { mutableStateOf(config.defaultPolicy.mode ?: "direct") }
    var recentFlow by remember { mutableStateOf<List<String>>(emptyList()) }

    LaunchedEffect(config.defaultPolicy.mode) {
        mode = config.defaultPolicy.mode ?: "direct"
    }

    // 实时流量流向：从引擎日志尾部提取最近 TCP/UDP 转发行
    LaunchedEffect(isConnected) {
        while (isConnected && isActive) {
            runCatching {
                val lines = mobile.Mobile.getLogs().lines().filter {
                    it.contains("[TCP]") || it.contains("[UDP]")
                }
                recentFlow = lines.takeLast(4)
            }
            delay(1500L)
        }
        if (!isConnected) recentFlow = emptyList()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("自动分流") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { inner ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(inner),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            item { SectionHeader(text = "流量流向") }
            item {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        if (recentFlow.isEmpty()) {
                            Text(
                                text = if (isConnected) "等待流量中..." else "代理未连接",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else {
                            recentFlow.forEach { line ->
                                Text(
                                    text = line.take(110),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.padding(vertical = 2.dp)
                                )
                            }
                        }
                    }
                }
            }

            item { SectionHeader(text = "自动分流策略") }
            item {
                Text(
                    text = "手动定义的「分流规则」条目拥有最高优先级。若均未命中，则按下方默认策略与内置 GFWList 回落处理。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            item {
                val defaultMode = listOf(
                    "tls-rf" to "TLS 分片（智能默认）",
                    "direct" to "直连",
                    "raw" to "原始直连",
                    "ttl-d" to "TTL 探测（desync）",
                    "block" to "阻断"
                )
                defaultMode.forEach { (id, label) ->
                    RadioOptionRow(
                        label = label,
                        selected = mode == id,
                        onClick = { mode = id }
                    )
                }
            }

            item { SectionHeader(text = "GFWList 状态检测") }
            item {
                InfoRowCard(
                    title = "内置规则源",
                    desc = "已预加载 GFWList 回落规则，规则源保持激活。"
                )
            }

            item { SectionHeader(text = "功能说明") }
            item { InfoRowCard(title = "智能模式", desc = "优先检测是否为 Cloudflare 站点，是则使用优选 IP 池，否则走 TLS 分片躲避 SNI 阻断。") }
            item { InfoRowCard(title = "优先级逻辑", desc = "手动定义的【分流规则】页面条目拥有最高优先级。若均未命中且分流开启，则走此处的自动逻辑。") }

            item {
                OutlinedButton(
                    onClick = {
                        viewModel.updateConfig(
                            config.copy(defaultPolicy = config.defaultPolicy.copy(mode = mode))
                        )
                        viewModel.saveConfig()
                    },
                    modifier = Modifier.fillMaxWidth()
                ) { Text("应用并保存设置") }
            }
        }
    }
}

@Composable
private fun InfoRowCard(title: String, desc: String) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Text(title, style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface)
            Spacer(modifier = Modifier.width(0.dp))
            Text(desc, style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 2.dp))
        }
    }
}
