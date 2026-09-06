package com.moi.lumine.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Save
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.model.Policy
import com.moi.lumine.network.NetworkMonitor
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader

private const val DEFAULT_NAT64_PREFIX = "2001:67c:2960:6464::"

// 上游(Upstream)字段允许 "ip:port" 输入，但 host 只接受单 IP/域名 → 去除端口。
private fun sanitizeHost(raw: String): String {
    val t = raw.trim()
    if (t.isEmpty()) return ""
    val hasSingleColon = t.count { it == ':' } == 1
    return if (hasSingleColon) t.substringBefore(':').trim() else t
}

private data class ModeOpt(val id: String, val label: String, val desc: String, val usable: Boolean)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RuleEditorScreen(navController: NavController, viewModel: ConfigViewModel, type: String) {
    val config by viewModel.currentConfig.collectAsState()
    val key by viewModel.editingRuleKey.collectAsState()
    val ruleKey = key

    DisposableEffect(Unit) {
        onDispose {
            viewModel.setEditingRule(null)
        }
    }

    if (ruleKey == null) {
        LaunchedEffect(Unit) {
            navController.popBackStack()
        }
        return
    }

    val existingPolicy = if (type == "domain") {
        config.domainPolicies[ruleKey]
    } else {
        config.ipPolicies[ruleKey]
    }
    val isNewRule = existingPolicy == null
    val initialPolicy = existingPolicy ?: Policy()
    val networkStatus by NetworkMonitor.status.collectAsState()
    val ipv6Available = networkStatus.ipv6Available != false

    var mode by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.mode ?: "tls-rf") }
    var host by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.host ?: "") }
    var mapTo by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.mapTo ?: "") }
    var tls13Only by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.tls13Only ?: false) }
    var nat64Enabled by remember(ruleKey, initialPolicy) {
        mutableStateOf(!initialPolicy.nat64Prefix.isNullOrBlank())
    }
    var nat64Prefix by remember(ruleKey, initialPolicy) {
        mutableStateOf(initialPolicy.nat64Prefix ?: DEFAULT_NAT64_PREFIX)
    }
    var dnsMode by remember(ruleKey, initialPolicy) {
        mutableStateOf(initialPolicy.dnsMode ?: "")
    }
    var numRecordsText by remember(ruleKey, initialPolicy) {
        mutableStateOf(initialPolicy.numRecords?.toString() ?: "")
    }
    var numSegsText by remember(ruleKey, initialPolicy) {
        mutableStateOf(initialPolicy.numSegs?.toString() ?: "")
    }
    var oob by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.oob ?: false) }
    var waitForAck by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.waitForAck ?: false) }
    var editableRuleKey by remember(ruleKey) { mutableStateOf(ruleKey) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        if (isNewRule) "新建规则" else "编辑规则",
                        style = MaterialTheme.typography.titleLarge
                    )
                },
                navigationIcon = {
                    IconButton(onClick = {
                        navController.popBackStack()
                    }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = {
                        val newKey = editableRuleKey.trim()
                        if (newKey.isEmpty()) return@IconButton
                        val updatedPolicy = initialPolicy.copy(
                            mode = mode,
                            host = sanitizeHost(host).ifEmpty { null },
                            mapTo = mapTo.ifEmpty { null },
                            dnsMode = dnsMode.ifEmpty { null },
                            tls13Only = tls13Only,
                            nat64Prefix = if (nat64Enabled && nat64Prefix.isNotBlank()) {
                                nat64Prefix.trim()
                            } else {
                                null
                            },
                            numRecords = numRecordsText.toIntOrNull()?.takeIf { it > 0 },
                            numSegs = numSegsText.toIntOrNull()?.takeIf { it > 0 },
                            oob = oob,
                            waitForAck = waitForAck
                        )
                        val updatedConfig = if (type == "domain") {
                            val newPolicies = if (newKey != ruleKey) {
                                config.domainPolicies - ruleKey + (newKey to updatedPolicy)
                            } else {
                                config.domainPolicies + (ruleKey to updatedPolicy)
                            }
                            config.copy(domainPolicies = newPolicies)
                        } else {
                            val newPolicies = if (newKey != ruleKey) {
                                config.ipPolicies - ruleKey + (newKey to updatedPolicy)
                            } else {
                                config.ipPolicies + (ruleKey to updatedPolicy)
                            }
                            config.copy(ipPolicies = newPolicies)
                        }
                        viewModel.updateConfig(updatedConfig)
                        viewModel.saveConfig()
                        navController.popBackStack()
                    }) {
                        Icon(Icons.Default.Save, contentDescription = "Save")
                    }
                }
            )
        }
    ) { innerPadding ->
        LazyColumn(
            modifier = Modifier
                .padding(innerPadding)
                .fillMaxSize()
                .padding(horizontal = 16.dp),
            contentPadding = PaddingValues(top = 16.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            item {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
                        contentColor = MaterialTheme.colorScheme.onSurface
                    ),
                    elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
                ) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        OutlinedTextField(
                            value = editableRuleKey,
                            onValueChange = { editableRuleKey = it },
                            label = { Text("规则路径") },
                            modifier = Modifier.fillMaxWidth(),
                            placeholder = {
                                Text(
                                    if (type == "domain") "例如 *.bing.com" else "例如 1.2.3.0/24"
                                )
                            },
                            singleLine = true
                        )
                    }
                }
            }

            item {
                SectionHeader(
                    text = "代理模式",
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
                )
            }

            val modeOptions = listOf(
                ModeOpt("tls-rf", "TLS 分片", "TLS 分片/记录重排，抗 SNI 阻断", true),
                ModeOpt("transparent", "透传", "不解密 TLS，仅透传", true),
                ModeOpt("raw", "原始直连", "不经任何处理的原始直连", true),
                ModeOpt("ttl-d", "TTL 探测", "TTL desync 抗封锁探测", true),
                ModeOpt("block", "阻断", "匹配后直接阻断连接", true),
                ModeOpt("mitm", "MITM", "需 CA 证书，移动端不支持", false),
                ModeOpt("quic", "QUIC 分片混淆", "桌面代理专有，移动端不支持", false),
                ModeOpt("migration", "会话恢复", "TLS 会话恢复迁移，桌面代理专有", false)
            )
            val modeValue = if (mode == "direct") "transparent" else mode
            items(modeOptions.size, key = { modeOptions[it].id }) { index ->
                val (id, label, desc, usable) = modeOptions[index]
                val selected = modeValue == id
                RadioOptionRow(
                    label = label,
                    selected = selected,
                    onClick = {
                        if (usable) {
                            mode = if (id == "transparent") "direct" else id
                        }
                    },
                    description = desc,
                    modifier = Modifier
                )
            }

            item {
                Spacer(modifier = Modifier.height(8.dp))
                SectionHeader(text = "上游服务器 (Upstream)")
                Spacer(modifier = Modifier.height(4.dp))
                OutlinedTextField(
                    value = host,
                    onValueChange = { host = it },
                    label = { Text("上游服务器 (Upstream)") },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("留空直连，或填写 IP / 域名（如 1.2.3.4）") },
                    supportingText = { Text("仅接受单个 IP 或域名；填写 ip:port 时仅取 IP 部分。") }
                )
                Spacer(modifier = Modifier.height(16.dp))
                OutlinedTextField(
                    value = mapTo,
                    onValueChange = { mapTo = it },
                    label = { Text("映射到 (Map To)") },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("例如 127.0.0.1:8080") }
                )
                Spacer(modifier = Modifier.height(8.dp))
                SectionHeader(text = "DNS 解析策略")
                Spacer(modifier = Modifier.height(4.dp))
                val dnsOptions = listOf(
                    "" to "默认",
                    "prefer_ipv4" to "IPv4 优先",
                    "prefer_ipv6" to "IPv6 优先",
                    "ipv4_only" to "仅 IPv4",
                    "ipv6_only" to "仅 IPv6"
                )
                dnsOptions.forEach { (id, label) ->
                    val v6Only = id == "prefer_ipv6" || id == "ipv6_only"
                    val dDisabled = v6Only && !ipv6Available
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable(enabled = !dDisabled) { dnsMode = id }
                            .padding(start = 4.dp, end = 16.dp, top = 2.dp, bottom = 2.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        RadioButton(selected = dnsMode == id, onClick = null, enabled = !dDisabled)
                        Spacer(modifier = Modifier.width(12.dp))
                        Column {
                            Text(
                                text = label,
                                style = MaterialTheme.typography.bodyLarge,
                                color = if (dDisabled) MaterialTheme.colorScheme.onSurfaceVariant
                                else MaterialTheme.colorScheme.onSurface
                            )
                            if (dDisabled) {
                                Text(
                                    text = "纯 IPv4 网络：IPv6 相关功能已禁用",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
                Spacer(modifier = Modifier.height(8.dp))
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { tls13Only = !tls13Only }
                        .padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Checkbox(checked = tls13Only, onCheckedChange = null)
                    Text(
                        text = "仅限 TLS 1.3",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
                Spacer(modifier = Modifier.height(16.dp))
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable {
                            nat64Enabled = !nat64Enabled
                            if (!nat64Enabled) {
                                nat64Prefix = ""
                            } else if (nat64Prefix.isBlank()) {
                                nat64Prefix = DEFAULT_NAT64_PREFIX
                            }
                        }
                        .padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Checkbox(checked = nat64Enabled, onCheckedChange = null)
                    Text(
                        text = "NAT64 (IPv4 → IPv6 映射)",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
                Text(
                    text = "把规则目标 IPv4 映射到 NAT64 前缀，叠加 tls-rf/ttl-d 等模式使用。需要出口具备真实 IPv6 连通性。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = nat64Prefix,
                    onValueChange = { nat64Prefix = it },
                    enabled = nat64Enabled,
                    label = { Text("NAT64 前缀") },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("2001:67c:2960:6464::") },
                    supportingText = {
                        Text(
                            "支持裸前缀或 CIDR（如 2001:67c:2960:6464::/96）。" +
                                "常用前缀：level66 2001:67c:2960:6464:: · nat64.net 2a01:4f9:c010:3f02:64:: · ZTVI 2602:fc59:11:64::"
                        )
                    }
                )
                Spacer(modifier = Modifier.height(24.dp))
                SectionHeader(text = "高级参数（可选）")
                Spacer(modifier = Modifier.height(4.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    OutlinedTextField(
                        value = numRecordsText,
                        onValueChange = { numRecordsText = it.filter(Char::isDigit) },
                        label = { Text("记录数 (num_records)") },
                        modifier = Modifier.weight(1f),
                        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                            keyboardType = androidx.compose.ui.text.input.KeyboardType.Number
                        ),
                        singleLine = true
                    )
                    OutlinedTextField(
                        value = numSegsText,
                        onValueChange = { numSegsText = it.filter(Char::isDigit) },
                        label = { Text("分段数 (num_segs)") },
                        modifier = Modifier.weight(1f),
                        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                            keyboardType = androidx.compose.ui.text.input.KeyboardType.Number
                        ),
                        singleLine = true
                    )
                }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { oob = !oob }
                        .padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Checkbox(checked = oob, onCheckedChange = null)
                    Text(
                        text = "OOB 分片（首包带外发送）",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { waitForAck = !waitForAck }
                        .padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Checkbox(checked = waitForAck, onCheckedChange = null)
                    Text(
                        text = "等待对端 ACK 后再发后续分片",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }
        }
    }
}
