package com.moi.lumine.ui.screens

import android.content.Context
import android.content.Intent
import android.graphics.drawable.BitmapDrawable
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.graphics.drawable.toBitmap
import androidx.navigation.NavController
import com.moi.lumine.repository.AppRoutingMode
import com.moi.lumine.repository.ConfigRepository
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader

private data class RoutingAppInfo(val label: String, val packageName: String)

private fun loadLaunchableApps(context: Context): List<RoutingAppInfo> {
    val pm = context.packageManager
    val intent = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
    val selfPackage = context.packageName
    return runCatching {
        pm.queryIntentActivities(intent, 0).orEmpty()
            .mapNotNull { info ->
                val pkg = info.activityInfo?.packageName ?: return@mapNotNull null
                if (pkg == selfPackage) return@mapNotNull null
                val label = info.loadLabel(pm)?.toString()?.takeIf { it.isNotBlank() } ?: pkg
                RoutingAppInfo(label = label, packageName = pkg)
            }
            .distinctBy { it.packageName }
            .sortedWith(compareBy(String.CASE_INSENSITIVE_ORDER) { it.label })
    }.getOrDefault(emptyList())
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AppRoutingScreen(navController: NavController, viewModel: ConfigViewModel) {
    val context = LocalContext.current
    val repository = remember { ConfigRepository(context) }

    var mode by remember { mutableStateOf(repository.getAppRoutingMode()) }
    var checked by remember { mutableStateOf(repository.getAppRoutingPackages()) }
    var query by remember { mutableStateOf("") }
    var manualInput by remember { mutableStateOf("") }

    val appList = remember(context) { loadLaunchableApps(context) }
    val keyword = query.trim()
    val filtered = remember(appList, keyword) {
        if (keyword.isEmpty()) {
            appList
        } else {
            appList.filter {
                it.label.contains(keyword, ignoreCase = true) ||
                    it.packageName.contains(keyword, ignoreCase = true)
            }
        }
    }
    val manualApps = remember(checked, appList, keyword) {
        checked.asSequence()
            .filter { pkg -> appList.none { it.packageName == pkg } }
            .filter { keyword.isEmpty() || it.contains(keyword, ignoreCase = true) }
            .map { RoutingAppInfo(label = it, packageName = it) }
            .sortedWith(compareBy(String.CASE_INSENSITIVE_ORDER) { it.packageName })
            .toList()
    }
    val modeLabel = when (mode) {
        AppRoutingMode.WHITELIST -> "白名单：仅所选应用走代理"
        AppRoutingMode.BYPASS -> "绕过名单：所选应用直连"
        AppRoutingMode.ALL -> "代理全部应用"
    }

    fun applyChange(nextMode: AppRoutingMode? = null, nextChecked: Set<String>? = null) {
        if (nextMode != null && nextMode != mode) {
            repository.setAppRoutingMode(nextMode)
            mode = nextMode
        }
        if (nextChecked != null) {
            repository.setAppRoutingPackages(nextChecked)
            checked = nextChecked
        }
    }

    fun addManualPackages() {
        val pkgs = manualInput.split(Regex("[\\s,;，；]+"))
            .map { it.trim() }
            .filter { it.isNotBlank() }
        if (pkgs.isEmpty()) {
            return
        }
        applyChange(nextChecked = checked + pkgs.toSet())
        manualInput = ""
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("分应用路由") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(horizontal = 16.dp)
        ) {
            SectionHeader(
                text = "路由方式",
                modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
            )
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.extraLarge,
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
                    contentColor = MaterialTheme.colorScheme.onSurface
                ),
                elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
            ) {
                Column(modifier = Modifier.padding(vertical = 4.dp)) {
                    RadioOptionRow(
                        label = "代理全部应用",
                        description = "所有应用流量经 Lumine 隧道转发",
                        selected = mode == AppRoutingMode.ALL,
                        onClick = { applyChange(nextMode = AppRoutingMode.ALL) }
                    )
                    RadioOptionRow(
                        label = "白名单模式",
                        description = "仅列表中选中的应用走代理，其余应用直连",
                        selected = mode == AppRoutingMode.WHITELIST,
                        onClick = { applyChange(nextMode = AppRoutingMode.WHITELIST) }
                    )
                    RadioOptionRow(
                        label = "绕过模式",
                        description = "列表中选中的应用直连，其余应用走代理",
                        selected = mode == AppRoutingMode.BYPASS,
                        onClick = { applyChange(nextMode = AppRoutingMode.BYPASS) }
                    )
                }
            }

            if (mode != AppRoutingMode.ALL) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "当前：$modeLabel · 已选 ${checked.size} 个应用",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.height(8.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(
                        value = query,
                        onValueChange = { query = it },
                        modifier = Modifier.weight(1f),
                        label = { Text("搜索应用") },
                        singleLine = true
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Column {
                        TextButton(
                            onClick = {
                                val next = checked + filtered.map { it.packageName }
                                applyChange(nextChecked = next)
                            }
                        ) {
                            Text("全选")
                        }
                        TextButton(
                            onClick = {
                                val removed = filtered.map { it.packageName }.toSet()
                                applyChange(nextChecked = checked - removed)
                            }
                        ) {
                            Text("清空筛选")
                        }
                    }
                }
                Spacer(modifier = Modifier.height(4.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(
                        value = manualInput,
                        onValueChange = { manualInput = it },
                        modifier = Modifier.weight(1f),
                        label = { Text("手动添加包名") },
                        placeholder = { Text("com.example.app，支持空格/逗号批量") },
                        singleLine = true
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    TextButton(
                        onClick = { addManualPackages() },
                        enabled = manualInput.isNotBlank()
                    ) {
                        Text("添加")
                    }
                }
                Spacer(modifier = Modifier.height(4.dp))

                if (mode == AppRoutingMode.WHITELIST && checked.isEmpty()) {
                    Text(
                        text = "未选择任何应用，VPN 建立后不会代理任何应用（等同关闭）。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }

                HorizontalDivider(
                    modifier = Modifier.padding(vertical = 8.dp),
                    color = MaterialTheme.colorScheme.outlineVariant
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            if (mode == AppRoutingMode.ALL) {
                Text(
                    text = "白名单/绕过名单仅作用于 VPN 建连时刻，修改后需重启连接生效。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(vertical = 8.dp)
                )
            } else {
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    if (manualApps.isNotEmpty()) {
                        item(key = "manual_header") {
                            Text(
                                text = "手动添加",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                            )
                        }
                        items(items = manualApps, key = { "manual:" + it.packageName }) { app ->
                            RoutingAppRow(
                                app = app,
                                selected = app.packageName in checked,
                                onClick = {
                                    applyChange(nextChecked = checked - app.packageName)
                                }
                            )
                        }
                    }
                    items(items = filtered, key = { it.packageName }) { app ->
                        RoutingAppRow(
                            app = app,
                            selected = app.packageName in checked,
                            onClick = {
                                val next = if (app.packageName in checked) {
                                    checked - app.packageName
                                } else {
                                    checked + app.packageName
                                }
                                applyChange(nextChecked = next)
                            }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun RoutingAppRow(app: RoutingAppInfo, selected: Boolean, onClick: () -> Unit) {
    val context = LocalContext.current
    val pm = context.packageManager
    val icon = remember(app.packageName) {
        runCatching {
            val drawable = pm.getApplicationIcon(app.packageName)
            if (drawable is BitmapDrawable) drawable.bitmap.asImageBitmap() else drawable.toBitmap().asImageBitmap()
        }.getOrNull()
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            modifier = Modifier.size(36.dp),
            contentAlignment = Alignment.Center
        ) {
            if (icon != null) {
                Image(
                    bitmap = icon,
                    contentDescription = null,
                    modifier = Modifier
                        .size(32.dp)
                        .padding(0.dp)
                )
            }
        }
        Spacer(modifier = Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = app.label,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = app.packageName,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        Checkbox(checked = selected, onCheckedChange = null)
    }
}
