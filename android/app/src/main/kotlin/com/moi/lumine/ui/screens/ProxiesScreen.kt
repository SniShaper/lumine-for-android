package com.moi.lumine.ui.screens

import android.content.Context
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
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Sync
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.network.NetworkMonitor
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.SectionHeader
import org.json.JSONObject

private const val PREFS_NAME = "lumine_profiles"
private const val KEY_NAT64 = "nat64_profiles"

private data class Nat64Profile(val id: String, val name: String, val prefix: String)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ProxiesScreen(navController: NavController, viewModel: ConfigViewModel) {
    val context = LocalContext.current
    val netStatus by NetworkMonitor.status.collectAsState()
    val ipv6Ok = netStatus.ipv6Available == true

    var profiles by remember { mutableStateOf(loadProfiles(context)) }
    var editing by remember { mutableStateOf<Nat64Profile?>(null) }
    var showAdd by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf<Nat64Profile?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }

    fun save(list: List<Nat64Profile>) {
        profiles = list
        storeProfiles(context, list)
        notice = "已保存"
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("代理") },
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
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            item {
                Text(
                    text = if (!ipv6Ok) "当前网络为纯 IPv4：NAT64 配置已禁用且不可点击，检测到 IPv6 后将自动恢复。" else
                        "配置独立规则前缀以便绕过特定黑名单封锁；在「分流规则 → 编辑 → NAT64」中可选用这些配置。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            item { SectionHeader(text = "NAT64 配置管理") }

            if (profiles.isEmpty()) {
                item {
                    Card(
                        modifier = Modifier.fillMaxWidth(),
                        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)
                    ) {
                        Text(
                            text = "暂无 NAT64 配置",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
            }
            items(profiles.size) { i ->
                val p = profiles[i]
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable(enabled = ipv6Ok) { editing = p },
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Surface(
                        modifier = Modifier.size(40.dp),
                        shape = CircleShape,
                        color = MaterialTheme.colorScheme.secondaryContainer
                    ) {
                        Icon(
                            imageVector = Icons.Default.Language,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSecondaryContainer,
                            modifier = Modifier.padding(8.dp)
                        )
                    }
                    Spacer(modifier = Modifier.width(14.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(p.name, style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurface)
                        Text("前缀：${p.prefix}", style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    IconButton(onClick = { editing = p }, enabled = ipv6Ok) {
                        Icon(Icons.Default.Edit, contentDescription = "编辑", tint = MaterialTheme.colorScheme.primary)
                    }
                    IconButton(onClick = { deleting = p }) {
                        Icon(Icons.Default.Delete, contentDescription = "删除",
                            tint = MaterialTheme.colorScheme.error)
                    }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            }

            item {
                OutlinedButton(
                    onClick = { showAdd = true },
                    enabled = ipv6Ok,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Icon(Icons.Default.Add, contentDescription = null)
                    Spacer(modifier = Modifier.width(6.dp))
                    Text("添加 NAT64 配置")
                }
            }

            item {
                Text(
                    text = notice ?: "",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.primary
                )
            }

            item { SectionHeader(text = "ECH 配置管理") }
            item {
                DisabledCard(
                    icon = Icons.Default.Lock,
                    title = "ECH 配置管理",
                    desc = "需要 CA 证书与桌面代理架构（MITM/ECH 终结），移动端不支持。"
                )
            }

            item { SectionHeader(text = "连接迁移服务") }
            item {
                DisabledCard(
                    icon = Icons.Default.Sync,
                    title = "连接迁移服务",
                    desc = "依赖 TLS 会话票据重写（桌面代理专有），移动端不支持。"
                )
            }
        }
    }

    if (showAdd) {
        ProfileDialog(
            initial = null,
            onDismiss = { showAdd = false },
            onConfirm = { name, prefix ->
                save(profiles + Nat64Profile(java.util.UUID.randomUUID().toString(), name, prefix))
                showAdd = false
            }
        )
    }
    editing?.let { p ->
        ProfileDialog(
            initial = p,
            onDismiss = { editing = null },
            onConfirm = { name, prefix ->
                save(profiles.map { if (it.id == p.id) it.copy(name = name, prefix = prefix) else it })
                editing = null
            }
        )
    }
    deleting?.let { p ->
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text("删除 NAT64 配置") },
            text = { Text("确定要删除此 NAT64 配置吗？") },
            confirmButton = {
                TextButton(onClick = {
                    save(profiles.filter { it.id != p.id })
                    deleting = null
                }) { Text("确定") }
            },
            dismissButton = {
                TextButton(onClick = { deleting = null }) { Text("取消") }
            }
        )
    }
}

@Composable
private fun DisabledCard(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String, desc: String) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerLow)
    ) {
        Row(modifier = Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.outline)
            Spacer(modifier = Modifier.width(14.dp))
            Column {
                Text(title, style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(desc, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f))
            }
        }
    }
}

@Composable
private fun ProfileDialog(
    initial: Nat64Profile?,
    onDismiss: () -> Unit,
    onConfirm: (name: String, prefix: String) -> Unit
) {
    var name by remember { mutableStateOf(initial?.name ?: "") }
    var prefix by remember { mutableStateOf(initial?.prefix ?: "2001:67c:2960:6464::") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (initial == null) "添加 NAT64 配置" else "编辑 NAT64 配置") },
        text = {
            Column {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("配置名称") },
                    placeholder = { Text("例如：特定黑名单绕过") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                OutlinedTextField(
                    value = prefix,
                    onValueChange = { prefix = it },
                    label = { Text("NAT64 前缀") },
                    placeholder = { Text("例如：64:ff9b::") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                    supportingText = { Text("支持裸前缀或 CIDR（如 /96）。常用：level66 2001:67c:2960:6464:: · nat64.net 2a01:4f9:c010:3f02:64::") }
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { if (name.isNotBlank() && prefix.isNotBlank()) onConfirm(name.trim(), prefix.trim()) },
                enabled = name.isNotBlank() && prefix.isNotBlank()
            ) { Text("保存") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("取消") } }
    )
}

private fun loadProfiles(context: Context): List<Nat64Profile> {
    val existing = runCatching {
        val raw = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .getString(KEY_NAT64, null) ?: return@runCatching null
        val arr = JSONObject(raw).optJSONArray("items") ?: return@runCatching null
        buildList<Nat64Profile> {
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                add(Nat64Profile(o.getString("id"), o.getString("name"), o.getString("prefix")))
            }
        }
    }.getOrNull()
    if (existing != null) return existing
    // 首次进入：预置桌面版 SniShaper 内置 NAT64 服务商
    val seeded = listOf(
        Nat64Profile("level66", "level66", "2001:67c:2960:6464::"),
        Nat64Profile("nat64.net", "nat64.net", "2a01:4f9:c010:3f02:64::"),
        Nat64Profile("ztvi", "ZTVI.org", "2602:fc59:11:64::")
    )
    storeProfiles(context, seeded)
    return seeded
}

private fun storeProfiles(context: Context, list: List<Nat64Profile>) {
    val arr = org.json.JSONArray()
    list.forEach {
        arr.put(JSONObject().put("id", it.id).put("name", it.name).put("prefix", it.prefix))
    }
    context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE).edit()
        .putString(KEY_NAT64, JSONObject().put("items", arr).toString()).apply()
}
