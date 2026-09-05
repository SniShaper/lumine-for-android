package com.moi.lumine.ui.screens

import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Save
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.network.NetworkMonitor
import com.moi.lumine.model.IPPool
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.io.File

private fun formatBytesSafe(bytes: Long): String {
    if (bytes < 0) return "-"
    if (bytes < 1024L) return "$bytes B"
    if (bytes < 1024L * 1024L) return String.format("%.1f KB", bytes / 1024f)
    return String.format("%.2f MB", bytes / (1024f * 1024f))
}

private fun formatUptime(ms: Long): String {
    if (ms <= 0) return "-"
    val s = ms / 1000
    val h = s / 3600
    val m = (s % 3600) / 60
    val sec = s % 60
    return if (h > 0) "${h}h${m}m" else if (m > 0) "${m}m${sec}s" else "${sec}s"
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GlobalSettingsScreen(navController: NavController, viewModel: ConfigViewModel) {
    val config by viewModel.currentConfig.collectAsState()
    val networkStatus by NetworkMonitor.status.collectAsState()

    var dnsAddr by remember { mutableStateOf(config.dns.addr) }
    var dnsType by remember { mutableStateOf(config.dns.type) }
    var dnsResolvers by remember {
        mutableStateOf((config.dns.resolvers ?: emptyList()).joinToString(" "))
    }
    var logLevel by remember { mutableStateOf(config.logLevel) }
    var defaultMode by remember { mutableStateOf(config.defaultPolicy.mode ?: "tls-rf") }
    var notice by remember { mutableStateOf<String?>(null) }
    var statsLine by remember { mutableStateOf("") }
    var logFilesLine by remember { mutableStateOf("") }
    var showAddPool by remember { mutableStateOf(false) }

    val context = LocalContext.current

    LaunchedEffect(config.dns.addr, config.dns.type, config.dns.resolvers, config.logLevel, config.defaultPolicy.mode) {
        dnsAddr = config.dns.addr
        dnsType = config.dns.type
        dnsResolvers = (config.dns.resolvers ?: emptyList()).joinToString(" ")
        logLevel = config.logLevel
        defaultMode = config.defaultPolicy.mode ?: "tls-rf"
    }

    LaunchedEffect(Unit) {
        while (isActive) {
            runCatching {
                val json = JSONObject(mobile.Mobile.getStats())
                val down = json.optLong("down", 0L)
                val up = json.optLong("up", 0L)
                val blocked = json.optLong("blocked", 0L)
                val tcp = json.optLong("tcp_conns", 0L)
                val udp = json.optLong("udp_conns", 0L)
                val uptime = json.optLong("uptime_ms", 0L)
                statsLine =
                    "↓ ${formatBytesSafe(down)}  ↑ ${formatBytesSafe(up)}  阻断 $blocked  TCP $tcp / UDP $udp  运行 ${formatUptime(uptime)}"
            }
            runCatching {
                val dir = File(context.filesDir, "logs")
                val files = dir.listFiles()?.filter { it.name.startsWith("lumine") } ?: emptyList()
                val total = files.sumOf { it.length() }
                logFilesLine = if (files.isEmpty()) {
                    "会话日志：未落盘（引擎运行后自动写入 filesDir/logs）"
                } else {
                    "日志文件：${files.size} 个，共 ${formatBytesSafe(total)}（logs/lumine*.log）"
                }
            }
            delay(2000L)
        }
    }

    val scope = rememberCoroutineScope()

    val importLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) {
            scope.launch {
                notice = viewModel.importConfigUri(uri) ?: "配置导入成功"
            }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("系统设置") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = {
                        val resolverList = dnsResolvers
                            .split(',', '，', ' ', '\n')
                            .map { it.trim() }
                            .filter { it.isNotBlank() }
                        val updatedConfig = config.copy(
                            dns = config.dns.copy(
                                type = dnsType,
                                addr = dnsAddr,
                                resolvers = resolverList.ifEmpty { null }
                            ),
                            logLevel = logLevel,
                            defaultPolicy = config.defaultPolicy.copy(mode = defaultMode)
                        )
                        viewModel.updateConfig(updatedConfig)
                        viewModel.saveConfig()
                        notice = "设置已保存"
                    }) {
                        Icon(Icons.Default.Save, contentDescription = "Save")
                    }
                }
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .padding(innerPadding)
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 8.dp)
        ) {
            if (notice != null) {
                Text(
                    text = notice ?: "",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.primary
                )
                Spacer(modifier = Modifier.height(8.dp))
            }

            SectionHeader(
                text = "核心设置",
                modifier = Modifier.padding(top = 8.dp, bottom = 8.dp)
            )

            Text(
                text = "DNS 传输类型",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Column(verticalArrangement = Arrangement.spacedBy(0.dp)) {
                val dnsTypes = listOf("https", "quic", "udp", "tcp", "tls")
                dnsTypes.forEach { t ->
                    RadioOptionRow(
                        label = t,
                        selected = (dnsType == t),
                        onClick = { dnsType = t }
                    )
                }
            }
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = if (dnsType == "quic") {
                    "quic = DoQ（RFC 9250，标准证书校验，默认端口 853）"
                } else if (dnsType == "https") {
                    "https = DoH（默认端口 443）"
                } else {
                    "其余类型地址须为 ip:port"
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            OutlinedTextField(
                value = dnsAddr,
                onValueChange = { dnsAddr = it },
                label = { Text("上游 DNS") },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("https://... 或 1.1.1.1:53") },
                supportingText = {
                    Text("DNS 解析使用的上游服务器地址")
                }
            )

            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = dnsResolvers,
                onValueChange = { dnsResolvers = it },
                label = { Text("备用 DNS 节点 (resolvers)") },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("https://public.dns.iij.jp/dns-query 1.1.1.1:53 …") },
                supportingText = {
                    Text("同类型多端点自动故障切换（粘住健康节点），空格/逗号分隔。")
                }
            )

            Spacer(modifier = Modifier.height(24.dp))

            SectionHeader(
                text = "日志级别",
                modifier = Modifier.padding(bottom = 4.dp)
            )

            Column(verticalArrangement = Arrangement.spacedBy(0.dp)) {
                val levels = listOf("DEBUG", "INFO", "WARN", "ERROR")
                levels.forEach { level ->
                    RadioOptionRow(
                        label = level,
                        selected = (logLevel == level),
                        onClick = { logLevel = level }
                    )
                }
            }

            Spacer(modifier = Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(modifier = Modifier.height(16.dp))

            SectionHeader(
                text = "默认策略模式",
                modifier = Modifier.padding(bottom = 4.dp)
            )
            Text(
                text = "未匹配任何规则时的处理方式（引擎 default_policy.mode）",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Column(verticalArrangement = Arrangement.spacedBy(0.dp)) {
                val modes = listOf("tls-rf", "direct", "raw", "ttl-d", "block")
                modes.forEach { m ->
                    RadioOptionRow(
                        label = m,
                        selected = (defaultMode == m),
                        onClick = { defaultMode = m }
                    )
                }
            }

            Spacer(modifier = Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(modifier = Modifier.height(16.dp))

            SectionHeader(
                text = "网络环境",
                modifier = Modifier.padding(bottom = 4.dp)
            )
            val netLine1 = when (networkStatus.ipv6Available) {
                true -> "IPv6：可用"
                false -> "IPv6：不可用（仅 IPv4）"
                null -> "IPv6：未知"
            }
            val netLine2 = networkStatus.nat64Prefix
                ?.takeIf { it.isNotBlank() }
                ?.let { "NAT64 前缀：$it" }
                ?: "NAT64：未检测到运营商前缀（可手动在规则中配置）"
            val netLine3 = networkStatus.lastEvent
                ?.let { "最近事件：$it" }
                ?: "最近事件：-"
            Text(
                text = "$netLine1\n$netLine2\n$netLine3",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedButton(onClick = {
                NetworkMonitor.refresh(context)
            }) {
                Text("重新检测")
            }

            Spacer(modifier = Modifier.height(16.dp))
            SectionHeader(
                text = "会话统计 / 日志文件",
                modifier = Modifier.padding(bottom = 4.dp)
            )
            Text(
                text = statsLine.ifBlank { "会话统计：引擎未运行" },
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = logFilesLine,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(modifier = Modifier.height(16.dp))

            val pools = config.ipPools ?: emptyMap()
            SectionHeader(
                text = "IP 池 (ip_pools)",
                modifier = Modifier.padding(bottom = 4.dp)
            )
            Text(
                text = "带 \$ 前缀引用（如 ip_policies 的 map_to: \$cloudflare）。修改在下次启动连接时生效。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            if (pools.isEmpty()) {
                Text(
                    text = "未配置 IP 池",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            } else {
                pools.forEach { (tag, pool) ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 2.dp),
                        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically
                    ) {
                        Text(
                            text = "\$$tag · ${pool.ips?.size ?: 0} IP · :${pool.port ?: 443}",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            modifier = Modifier.weight(1f)
                        )
                        TextButton(onClick = {
                            val next = pools - tag
                            viewModel.updateConfig(config.copy(ipPools = next.ifEmpty { null }))
                            viewModel.saveConfig()
                            notice = "已删除 IP 池 $tag"
                        }) {
                            Text("删除")
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(4.dp))
            OutlinedButton(onClick = { showAddPool = true }) {
                Text("添加 / 覆盖 IP 池")
            }

            Spacer(modifier = Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(modifier = Modifier.height(16.dp))

            SectionHeader(
                text = "配置导入 / 导出",
                modifier = Modifier.padding(bottom = 4.dp)
            )
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedButton(onClick = {
                    importLauncher.launch(arrayOf("application/json", "text/plain", "*/*"))
                }) {
                    Text("导入配置")
                }
                OutlinedButton(onClick = {
                    scope.launch {
                        try {
                            val exported = viewModel.exportCurrentConfig()
                            val shareIntent = Intent(Intent.ACTION_SEND).apply {
                                type = "application/json"
                                putExtra(Intent.EXTRA_STREAM, exported.uri)
                                putExtra(Intent.EXTRA_TEXT, exported.fileName)
                                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                            }
                            context.startActivity(
                                Intent.createChooser(shareIntent, "导出配置")
                            )
                        } catch (e: Exception) {
                            notice = "导出失败：${e.message}"
                        }
                    }
                }) {
                    Text("导出配置")
                }
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "导入会以新的独立配置保存并自动切换；导出为完整配置 JSON（含引擎全部字段）。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(24.dp))
        }
    }

    if (showAddPool) {
        AddPoolDialog(
            onDismiss = { showAddPool = false },
            onConfirm = { tag, ips, port ->
                val cur = config.ipPools ?: emptyMap()
                val next = cur + (tag to IPPool(ips = ips, port = port))
                viewModel.updateConfig(config.copy(ipPools = next))
                viewModel.saveConfig()
                showAddPool = false
                notice = "已保存 IP 池 $tag"
            }
        )
    }
}

@Composable
private fun AddPoolDialog(
    onDismiss: () -> Unit,
    onConfirm: (tag: String, ips: List<String>, port: Int) -> Unit
) {
    var tag by remember { mutableStateOf("") }
    var ips by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("443") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("IP 池") },
        text = {
            Column {
                OutlinedTextField(
                    value = tag,
                    onValueChange = { tag = it },
                    label = { Text("标签") },
                    singleLine = true,
                    placeholder = { Text("cloudflare") }
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = ips,
                    onValueChange = { ips = it },
                    label = { Text("IP / 域名（逗号分隔）") },
                    placeholder = { Text("1.1.1.1, 104.17.0.1, cdn.example.com") }
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = port,
                    onValueChange = { port = it.filter(Char::isDigit) },
                    label = { Text("探测端口") },
                    singleLine = true
                )
            }
        },
        confirmButton = {
            TextButton(
                enabled = tag.isNotBlank() && ips.isNotBlank(),
                onClick = {
                    val parsedPort = port.toIntOrNull()?.takeIf { it in 1..65535 } ?: 443
                    val items = ips.split(',', ';', '，')
                        .map { it.trim() }
                        .filter { it.isNotBlank() }
                    onConfirm(tag.trim(), items, parsedPort)
                }
            ) {
                Text("保存")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("取消")
            }
        }
    )
}
