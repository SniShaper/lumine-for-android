package com.moi.lumine.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.network.NetworkMonitor
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.SectionHeader
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.net.UnknownHostException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private data class EvoEntry(
    val domain: String,
    val method: String,      // 直连 / TLS分片(经NAT64) / 不可达
    val delayMs: Long?,
    val ok: Boolean
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EvolutionScreen(navController: NavController, viewModel: ConfigViewModel) {
    val config by viewModel.currentConfig.collectAsState()
    val netStatus by NetworkMonitor.status.collectAsState()
    val scope = rememberCoroutineScope()

    var tab by remember { mutableStateOf(0) }
    var domainsText by remember { mutableStateOf("") }
    var enableIpv6 by remember { mutableStateOf(false) }
    var running by remember { mutableStateOf(false) }
    var progress by remember { mutableStateOf(0) }
    var total by remember { mutableStateOf(0) }
    var entries by remember { mutableStateOf<List<EvoEntry>>(emptyList()) }
    var tempRules by remember { mutableStateOf<Set<String>>(emptySet()) }
    var converting by remember { mutableStateOf<String?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }
    val nat64Prefix = netStatus.nat64Prefix ?: ""

    fun startTest() {
        val list = domainsText.split(Regex("[\\s,;\\n]+")).map { it.trim().lowercase() }.filter { it.isNotEmpty() }
        if (list.isEmpty() || running) return
        running = true
        total = list.size
        entries = emptyList()
        tempRules = emptySet()
        notice = "开始测试 ${list.size} 个域名"
        scope.launch {
            val results = mutableListOf<EvoEntry>()
            for ((idx, domain) in list.withIndex()) {
                progress = idx
                val direct = withContext(Dispatchers.IO) { probeTcp(domain, 443) }
                if (direct != null) {
                    results += EvoEntry(domain, "直连", direct, true)
                } else {
                    // NAT64 映射探测（需真实出口 IPv6 或 v4 映射通道）
                    var mapped: Long? = null
                    if (enableIpv6 && nat64Prefix.isNotBlank()) {
                        mapped = withContext(Dispatchers.IO) { probeNat64(domain, nat64Prefix) }
                    }
                    if (mapped != null) {
                        results += EvoEntry(domain, "TLS分片 (NAT64)", mapped, true)
                    } else {
                        results += EvoEntry(domain, "不可达", null, false)
                    }
                }
            }
            progress = list.size
            entries = results
            tempRules = results.filter { it.ok }.map { it.domain }.toSet()
            running = false
            notice = "测试完成，共 ${results.size} 个域名"
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text("进化模式")
                        Text("自动测试域名连通性并生成最优规则",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { inner ->
        Column(modifier = Modifier.fillMaxSize().padding(inner)) {
            TabRow(selectedTabIndex = tab) {
                listOf("测试", "规则", "结果").forEachIndexed { i, label ->
                    Tab(
                        selected = tab == i,
                        onClick = { tab = i },
                        text = { Text(label) }
                    )
                }
            }
            when (tab) {
                0 -> TestTab(
                    domainsText = domainsText,
                    onDomainsChange = { domainsText = it },
                    enableIpv6 = enableIpv6,
                    onIpv6Change = { enableIpv6 = it },
                    ipv6Usable = netStatus.ipv6Available == true || nat64Prefix.isNotBlank(),
                    running = running,
                    progress = progress,
                    total = total,
                    onStart = { startTest() },
                    onStop = { running = false; notice = "测试已停止" }
                )
                1 -> RulesTab(
                    config = config,
                    okDomains = tempRules,
                    entries = entries,
                    onConvert = { converting = it }
                )
                2 -> ResultsTab(entries)
            }
            notice?.let {
                Text(it, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
            }
        }
    }

    converting?.let { domain ->
        val entry = entries.firstOrNull { it.domain == domain }
        AlertDialog(
            onDismissRequest = { converting = null },
            title = { Text("确认转为正式规则") },
            text = { Text("确定要将以下规则转为正式规则并写入配置吗？") },
            confirmButton = {
                TextButton(onClick = {
                    entry?.let { e ->
                        val dp = config.domainPolicies.toMutableMap()
                        dp[e.domain] = com.moi.lumine.model.Policy(mode = "direct")
                        viewModel.updateConfig(config.copy(domainPolicies = dp))
                        viewModel.saveConfig()
                    }
                    tempRules = tempRules - domain
                    converting = null
                    notice = "规则 \"$domain\" 已转为正式规则"
                }) { Text("确认转换") }
            },
            dismissButton = { TextButton(onClick = { converting = null }) { Text("取消") } }
        )
    }
}

@Composable
private fun TestTab(
    domainsText: String,
    onDomainsChange: (String) -> Unit,
    enableIpv6: Boolean,
    onIpv6Change: (Boolean) -> Unit,
    ipv6Usable: Boolean,
    running: Boolean,
    progress: Int,
    total: Int,
    onStart: () -> Unit,
    onStop: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item {
            SectionHeader(text = "域名列表")
            val count = domainsText.split(Regex("[\\s,;\\n]+")).count { it.isNotBlank() }
            Text("$count 个", style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        item {
            OutlinedTextField(
                value = domainsText,
                onValueChange = onDomainsChange,
                modifier = Modifier.fillMaxWidth(),
                minLines = 4,
                placeholder = { Text("example.com\ntest.com\ndemo.org") },
                supportingText = { Text("每行一个域名，支持批量粘贴") }
            )
        }
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text("启用 IPv6", style = MaterialTheme.typography.bodyLarge)
                    Text(
                        text = if (!ipv6Usable) "当前网络为纯 IPv4（未检测到 NAT64 前缀），不可用" else "不可达域名将通过 NAT64 前缀再次探测",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Switch(
                    checked = enableIpv6,
                    onCheckedChange = { if (ipv6Usable) onIpv6Change(it) },
                    enabled = ipv6Usable
                )
            }
        }
            val frac = if (total > 0) progress.toFloat() / total else 0f
            if (running) {
                item {
                    SectionHeader(text = "测试进度")
                    LinearProgressIndicator(
                        progress = { frac },
                        modifier = Modifier.fillMaxWidth()
                    )
                    Text(
                        text = if (total > progress) "正在测试第 ${progress + 1} 个域名..." else "测试完成",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(top = 4.dp)
                    )
                }
            }
        item {
            SectionHeader(text = "测试流程")
            Text("直连探测 → NAT64 探测 → 生成临时规则；成功后自动生成临时规则，在「规则」页面可转为正式规则",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        item {
            Button(
                onClick = onStart,
                enabled = !running && domainsText.isNotBlank(),
                modifier = Modifier.fillMaxWidth()
            ) { Text("开始测试") }
        }
    }
}

@Composable
private fun RulesTab(
    config: com.moi.lumine.model.LumineConfig,
    okDomains: Set<String>,
    entries: List<EvoEntry>,
    onConvert: (String) -> Unit
) {
    val converted = entries.filter { it.ok }.count { it.domain in config.domainPolicies }
    val pending = okDomains - config.domainPolicies.keys.toSet()
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        if (pending.isEmpty()) {
            item {
                Text("暂无生成的规则",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text("完成测试后，成功的域名将生成临时规则",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        item {
            SectionHeader(text = "临时规则")
            val totalOk = entries.count { it.ok }
            Text("已应用: ${converted} / $totalOk",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        items(pending.toList().sorted()) { domain ->
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(domain, style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurface)
                        Text("直连", style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    OutlinedButton(onClick = { onConvert(domain) }) { Text("转为正式") }
                }
            }
        }
    }
}

@Composable
private fun ResultsTab(entries: List<EvoEntry>) {
    val ok = entries.count { it.ok }
    val fail = entries.size - ok
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        if (entries.isEmpty()) {
            item {
                Text("暂无测试结果", style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text("在「测试」页面输入域名并开始测试", style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        } else {
            item {
                SectionHeader(text = "测试结果")
                Text("$ok 成功 · $fail 失败", style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            items(entries) { e ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(e.domain, style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface,
                        modifier = Modifier.weight(1f))
                    Text(
                        text = when {
                            !e.ok -> "不可达"
                            e.delayMs != null -> "${e.delayMs}ms"
                            else -> ""
                        },
                        style = MaterialTheme.typography.bodySmall,
                        color = if (e.ok) MaterialTheme.colorScheme.onSurfaceVariant
                        else MaterialTheme.colorScheme.error
                    )
                }
            }
        }
    }
}

private suspend fun probeTcp(host: String, port: Int): Long? {
    return withContext(Dispatchers.IO) {
        try {
            val start = System.nanoTime()
            Socket().use { s ->
                s.connect(InetSocketAddress(host, port), 3000)
                (System.nanoTime() - start) / 1_000_000
            }
        } catch (e: Exception) {
            null
        }
    }
}

private suspend fun probeNat64(domain: String, prefix: String): Long? {
    return withContext(Dispatchers.IO) {
        try {
            val v4 = InetAddress.getAllByName(domain).firstOrNull { it is Inet4Address } ?: return@withContext null
            val mapped = mapToNat64(v4, prefix) ?: return@withContext null
            val start = System.nanoTime()
            Socket().use { s ->
                s.connect(InetSocketAddress(mapped, 443), 3000)
                (System.nanoTime() - start) / 1_000_000
            }
        } catch (e: Exception) {
            null
        }
    }
}

private fun mapToNat64(v4: InetAddress, prefix: String): InetAddress? {
    return runCatching {
        var p = prefix.trim().removeSuffix("/")
        if ('/' in p) p = p.substringBefore("/")
        val prefixBytes = InetAddress.getByName(p).address
        if (prefixBytes.size != 16) return null
        val v4Bytes = v4.address
        val out = ByteArray(16)
        System.arraycopy(prefixBytes, 0, out, 0, 12)
        System.arraycopy(v4Bytes, 0, out, 12, 4)
        Inet6Address.getByAddress(null, out, 0)
    }.getOrNull()
}
