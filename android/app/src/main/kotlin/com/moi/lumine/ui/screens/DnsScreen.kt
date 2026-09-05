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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
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
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.SectionHeader
import java.net.InetSocketAddress
import java.net.Socket
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DnsScreen(navController: NavController, viewModel: ConfigViewModel) {
    val config by viewModel.currentConfig.collectAsState()

    // 节点 = 主节点(dns.addr) + 备用(dns.resolvers)；全局传输类型决定协议。
    val typeOptions = listOf(
        "https" to "DoH (HTTPS)",
        "quic" to "DoQ (QUIC)",
        "tcp" to "TCP",
        "tls" to "DoT (TLS)",
        "udp" to "UDP"
    )
    var type by remember { mutableStateOf(config.dns.type) }
    var nodes by remember {
        mutableStateOf(listOf(config.dns.addr) + (config.dns.resolvers ?: emptyList()))
    }
    var editingIdx by remember { mutableStateOf<Int?>(null) }
    var showAdd by remember { mutableStateOf(false) }
    var deletingIdx by remember { mutableStateOf<Int?>(null) }
    var testing by remember { mutableStateOf(false) }
    var results by remember { mutableStateOf<Map<Int, String>>(emptyMap()) }
    var notice by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(config.dns.addr, config.dns.resolvers, config.dns.type) {
        type = config.dns.type
        nodes = listOf(config.dns.addr) + (config.dns.resolvers ?: emptyList())
    }

    fun persist(nextType: String, list: List<String>) {
        if (list.isEmpty()) return
        val updated = config.copy(
            dns = config.dns.copy(type = nextType, addr = list.first(), resolvers = list.drop(1).ifEmpty { null })
        )
        viewModel.updateConfig(updated)
        viewModel.saveConfig()
        notice = "DNS 节点已更新"
    }

    fun testAll(list: List<String>, nextType: String) {
        testing = true
        scope.launch {
            val out = mutableMapOf<Int, String>()
            for ((i, node) in list.withIndex()) {
                val (host, port) = parseNode(node, nextType)
                val rtt = withContext(Dispatchers.IO) { tcpRtt(host, port) }
                out[i] = if (rtt != null) "${rtt}ms" else "不可达"
            }
            results = out
            testing = false
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("安全 DNS") },
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
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            item {
                Text(
                    text = "第一个节点为主节点，其余为备用节点；解析失败会自动切换到下一个可用节点。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            item { SectionHeader(text = "传输类型") }
            item {
                typeOptions.forEach { (id, label) ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable {
                                type = id
                                persist(id, nodes)
                            },
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        RadioButton(selected = type == id, onClick = null)
                        Text(label, style = MaterialTheme.typography.bodyLarge)
                    }
                }
            }
            item { SectionHeader(text = "DNS 节点") }
            if (nodes.isEmpty()) {
                item {
                    Text("暂无 DNS 节点", style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(vertical = 8.dp))
                }
            }
            items(nodes.size) { i ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { editingIdx = i },
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Surface(
                        modifier = Modifier.size(40.dp),
                        shape = CircleShape,
                        color = MaterialTheme.colorScheme.secondaryContainer
                    ) {
                        Icon(Icons.Default.Dns, contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSecondaryContainer,
                            modifier = Modifier.padding(8.dp))
                    }
                    Spacer(modifier = Modifier.width(14.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                if (i == 0) "主节点" else "备用 ${i}",
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.primary
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(nodes[i], style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface,
                                maxLines = 1)
                        }
                        val st = results[i]
                        if (st != null) {
                            Text(st, style = MaterialTheme.typography.bodySmall,
                                color = if (st == "不可达") MaterialTheme.colorScheme.error
                                else MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    if (i > 0) {
                        IconButton(onClick = {
                            val list = nodes.toMutableList()
                            val v = list.removeAt(i)
                            list.add(i - 1, v)
                            nodes = list
                            persist(type, list)
                        }) {
                            Icon(Icons.Default.KeyboardArrowUp, contentDescription = "上移")
                        }
                    }
                    if (i < nodes.lastIndex) {
                        IconButton(onClick = {
                            val list = nodes.toMutableList()
                            val v = list.removeAt(i)
                            list.add(i + 1, v)
                            nodes = list
                            persist(type, list)
                        }) {
                            Icon(Icons.Default.KeyboardArrowDown, contentDescription = "下移")
                        }
                    }
                    IconButton(onClick = { editingIdx = i }) {
                        Icon(Icons.Default.Edit, contentDescription = "编辑", tint = MaterialTheme.colorScheme.primary)
                    }
                    IconButton(onClick = { deletingIdx = i }) {
                        Icon(Icons.Default.Delete, contentDescription = "删除", tint = MaterialTheme.colorScheme.error)
                    }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            }
            item {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    OutlinedButton(onClick = { showAdd = true }, modifier = Modifier.weight(1f)) {
                        Icon(Icons.Default.Add, contentDescription = null)
                        Spacer(modifier = Modifier.width(4.dp))
                        Text("添加节点")
                    }
                    OutlinedButton(
                        onClick = { testAll(nodes, type) },
                        enabled = nodes.isNotEmpty() && !testing,
                        modifier = Modifier.weight(1f)
                    ) { Text(if (testing) "测试中..." else "全部测试") }
                }
            }
            item {
                Text(notice ?: "", style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.primary)
            }
        }
    }

    if (showAdd) {
        NodeDialog(
            title = "添加 DNS 节点",
            initial = "",
            presets = DNS_PRESETS,
            onDismiss = { showAdd = false },
            onConfirm = { v ->
                val list = nodes + v
                nodes = list
                persist(type, list)
                showAdd = false
            }
        )
    }
    editingIdx?.let { idx ->
        NodeDialog(
            title = "编辑 DNS 节点",
            initial = nodes.getOrElse(idx) { "" },
            presets = DNS_PRESETS,
            onDismiss = { editingIdx = null },
            onConfirm = { v ->
                val list = nodes.toMutableList()
                if (idx < list.size) list[idx] = v
                nodes = list
                persist(type, list)
                editingIdx = null
            }
        )
    }
    deletingIdx?.let { idx ->
        AlertDialog(
            onDismissRequest = { deletingIdx = null },
            title = { Text("删除 DNS 节点") },
            text = { Text("移除后将不再使用该节点进行 DNS 解析。") },
            confirmButton = {
                TextButton(onClick = {
                    val list = nodes.toMutableList()
                    if (idx < list.size) {
                        list.removeAt(idx)
                        if (list.isNotEmpty()) {
                            nodes = list
                            persist(type, list)
                        } else {
                            notice = "至少需要保留一个 DNS 节点"
                        }
                    }
                    deletingIdx = null
                }) { Text("确定") }
            },
            dismissButton = { TextButton(onClick = { deletingIdx = null }) { Text("取消") } }
        )
    }
}

@Composable
private fun NodeDialog(
    title: String,
    initial: String,
    presets: List<Pair<String, String>>,
    onDismiss: () -> Unit,
    onConfirm: (value: String) -> Unit
) {
    var value by remember { mutableStateOf(initial) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                OutlinedTextField(
                    value = value,
                    onValueChange = { value = it },
                    label = { Text("DoH URL / 服务器地址") },
                    placeholder = { Text("https://dns.example.com/dns-query  或  1.1.1.1:853") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Text(
                    text = "QUIC/TLS 端点省略端口时默认 853；DoH 需完整 URL。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 6.dp)
                )
                Text(
                    text = "常用服务商",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.padding(top = 10.dp)
                )
                presets.forEach { (label, preset) ->
                    Text(
                        text = "· $label",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { value = preset }
                            .padding(vertical = 3.dp)
                    )
                }
                Text(
                    text = "提示：IIJ DoQ 节点需在传输类型中选择 QUIC；Cloudflare Gateway 为 DoH。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp)
                )
            }
        },
        confirmButton = {
            TextButton(onClick = { onConfirm(value.trim()) }, enabled = value.isNotBlank()) { Text("保存") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("取消") } }
    )
}

private val DNS_PRESETS: List<Pair<String, String>> = listOf(
    "Cloudflare Gateway (DoH)" to "https://xwfpeb16ii.cloudflare-gateway.com/dns-query",
    "IIJ DoQ (QUIC, DoQ)" to "public.dns.iij.jp:853"
)

private fun parseNode(node: String, type: String): Pair<String, Int> {
    val raw = node.trim()
    // URL 形式: 取 host 并处理 /dns-query 路径
    val host = raw.substringAfter("://").substringBefore("/")
    if (host.contains(":")) {
        val h = host.substringBeforeLast(":")
        val p = host.substringAfterLast(":")
        p.toIntOrNull()?.let { return h to it }
    }
    val defaultPort = if (type == "https") 443 else 853
    return host to defaultPort
}

private suspend fun tcpRtt(host: String, port: Int): Long? {
    return try {
        withContext(Dispatchers.IO) {
            val start = System.nanoTime()
            Socket().use { s ->
                s.connect(InetSocketAddress(host, port), 3000)
                (System.nanoTime() - start) / 1_000_000
            }
        }
    } catch (e: Exception) {
        null
    }
}
