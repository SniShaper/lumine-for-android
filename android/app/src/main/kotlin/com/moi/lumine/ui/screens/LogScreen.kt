@file:OptIn(androidx.compose.foundation.layout.ExperimentalLayoutApi::class)

package com.moi.lumine.ui.screens

import android.content.ActivityNotFoundException
import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.BugReport
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.RuntimeLogEntry
import com.moi.lumine.RuntimeLogLevel
import com.moi.lumine.ui.ConfigViewModel

private const val LOG_LIST_HEADER_COUNT = 2

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun LogScreen(navController: NavController, viewModel: ConfigViewModel) {
    val context = LocalContext.current
    val logSnapshot by viewModel.logSnapshot.collectAsState()
    val isLogCaptureEnabled by viewModel.isLogCaptureEnabled.collectAsState()
    val vpnStatus by viewModel.vpnStatus.collectAsState()
    val exportEvent by viewModel.logExportEvent.collectAsState()
    val exportMessage by viewModel.logExportMessage.collectAsState()
    val isExportingLogs by viewModel.isExportingLogs.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    val listState = rememberLazyListState()
    var isAutoScrollEnabled by remember { mutableStateOf(true) }
    var selectedLevel by remember { mutableStateOf(LogLevelFilter.All) }
    val allLogs = logSnapshot.entries
    val filteredLogs = remember(allLogs, selectedLevel) {
        if (selectedLevel == LogLevelFilter.All) {
            allLogs
        } else {
            allLogs.filter { selectedLevel.matches(it.level) }
        }
    }
    val shouldFollowTail by remember(listState, filteredLogs, isAutoScrollEnabled) {
        derivedStateOf {
            if (!isAutoScrollEnabled || filteredLogs.isEmpty()) {
                false
            } else {
                val targetIndex = filteredLogs.lastIndex + LOG_LIST_HEADER_COUNT
                val lastVisibleIndex = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0
                listState.layoutInfo.totalItemsCount == 0 ||
                    targetIndex - lastVisibleIndex <= 3 ||
                    !listState.canScrollForward
            }
        }
    }

    LaunchedEffect(filteredLogs.lastOrNull()?.id, selectedLevel, isAutoScrollEnabled) {
        if (shouldFollowTail && filteredLogs.isNotEmpty()) {
            listState.scrollToItem(filteredLogs.lastIndex + LOG_LIST_HEADER_COUNT)
        }
    }

    LaunchedEffect(exportMessage) {
        val current = exportMessage ?: return@LaunchedEffect
        snackbarHostState.showSnackbar(current)
        viewModel.consumeLogExportMessage()
    }

    LaunchedEffect(exportEvent) {
        val payload = exportEvent ?: return@LaunchedEffect
        try {
            val shareIntent = Intent(Intent.ACTION_SEND).apply {
                type = "text/plain"
                putExtra(Intent.EXTRA_STREAM, payload.uri)
                putExtra(Intent.EXTRA_SUBJECT, "Lumine 日志 ${payload.fileName}")
                putExtra(Intent.EXTRA_TEXT, "Lumine 导出日志: ${payload.fileName}")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            context.startActivity(Intent.createChooser(shareIntent, "导出日志"))
        } catch (_: ActivityNotFoundException) {
            viewModel.reportLogExportMessage("没有可用的分享应用")
        } finally {
            viewModel.consumeLogExportEvent()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("实时日志") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { viewModel.toggleLogCapture() }) {
                        Icon(
                            Icons.Default.BugReport,
                            contentDescription = if (isLogCaptureEnabled) "停止捕捉日志" else "开始捕捉日志",
                            tint = if (isLogCaptureEnabled) {
                                Color(0xFFB3261E)
                            } else {
                                MaterialTheme.colorScheme.onSurfaceVariant
                            }
                        )
                    }
                    IconButton(onClick = { isAutoScrollEnabled = !isAutoScrollEnabled }) {
                        Icon(
                            if (isAutoScrollEnabled) Icons.Default.Pause else Icons.Default.PlayArrow,
                            contentDescription = "Toggle Auto-scroll"
                        )
                    }
                    IconButton(
                        onClick = { viewModel.exportLogs() },
                        enabled = !isExportingLogs
                    ) {
                        if (isExportingLogs) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                strokeWidth = 2.dp
                            )
                        } else {
                            Icon(Icons.Default.Share, contentDescription = "Export Logs")
                        }
                    }
                    IconButton(onClick = { viewModel.clearLogs() }) {
                        Icon(Icons.Default.DeleteSweep, contentDescription = "Clear Logs")
                    }
                }
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { innerPadding ->
        LazyColumn(
            state = listState,
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            item {
                LogSummaryCard(
                    statusMessage = vpnStatus.message,
                    isLogCaptureEnabled = isLogCaptureEnabled,
                    totalCount = logSnapshot.totalCount,
                    infoCount = logSnapshot.infoCount,
                    errorCount = logSnapshot.errorCount,
                    debugCount = logSnapshot.debugCount,
                    onToggleCapture = { viewModel.toggleLogCapture() },
                    onExport = { viewModel.exportLogs() }
                )
            }

            item {
                LogFilterRow(
                    selectedLevel = selectedLevel,
                    onSelectedChange = { selectedLevel = it }
                )
            }

            if (filteredLogs.isEmpty()) {
                item {
                    EmptyLogState(
                        hasAnyLogs = allLogs.isNotEmpty(),
                        isRunning = vpnStatus.phase == "running" || vpnStatus.phase == "starting"
                    )
                }
            } else {
                items(filteredLogs, key = { it.id }) { log ->
                    LogItem(log)
                }
            }
        }
    }
}

@Composable
private fun LogSummaryCard(
    statusMessage: String,
    isLogCaptureEnabled: Boolean,
    totalCount: Int,
    infoCount: Int,
    errorCount: Int,
    debugCount: Int,
    onToggleCapture: () -> Unit,
    onExport: () -> Unit
) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = "运行日志",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold
            )
            Text(
                text = statusMessage,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            AssistChip(
                onClick = onToggleCapture,
                label = { Text(if (isLogCaptureEnabled) "日志捕捉中" else "日志捕捉未开启") },
                leadingIcon = {
                    Icon(
                        imageVector = Icons.Default.BugReport,
                        contentDescription = null,
                        tint = if (isLogCaptureEnabled) {
                            Color(0xFFB3261E)
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        }
                    )
                }
            )
            AssistChip(
                onClick = onExport,
                label = { Text("导出当前日志") },
                leadingIcon = {
                    Icon(
                        imageVector = Icons.Default.Share,
                        contentDescription = null
                    )
                }
            )
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                SummaryChip(label = "总计 $totalCount", color = MaterialTheme.colorScheme.primaryContainer)
                SummaryChip(label = "信息 $infoCount", color = Color(0xFFDDF4E4))
                SummaryChip(label = "错误 $errorCount", color = Color(0xFFFFE0E0))
                SummaryChip(label = "调试 $debugCount", color = Color(0xFFE7EAF3))
            }
        }
    }
}

@Composable
private fun SummaryChip(label: String, color: Color) {
    Surface(
        color = color,
        shape = MaterialTheme.shapes.small
    ) {
        Text(
            text = label,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
            style = MaterialTheme.typography.labelMedium
        )
    }
}

@Composable
private fun LogFilterRow(
    selectedLevel: LogLevelFilter,
    onSelectedChange: (LogLevelFilter) -> Unit
) {
    FlowRow(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        LogLevelFilter.entries.forEach { filter ->
            FilterChip(
                selected = selectedLevel == filter,
                onClick = { onSelectedChange(filter) },
                label = { Text(filter.label) }
            )
        }
    }
}

@Composable
private fun EmptyLogState(hasAnyLogs: Boolean, isRunning: Boolean) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            if (isRunning) {
                CircularProgressIndicator(modifier = Modifier.size(28.dp), strokeWidth = 2.5.dp)
            }
            Text(
                text = if (hasAnyLogs) "当前筛选条件下没有日志" else "还没有收到核心日志",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold
            )
            Text(
                text = if (hasAnyLogs) "切换日志级别后再看一次。" else "点击右上角的捕捉按钮后，这里才会开始收集核心日志。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun LogItem(log: RuntimeLogEntry) {
    val visuals = remember(log.level) { log.level.visuals() }
    Card(
        colors = CardDefaults.cardColors(containerColor = visuals.background)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Icon(
                imageVector = visuals.icon,
                contentDescription = null,
                tint = visuals.accent,
                modifier = Modifier.padding(top = 2.dp)
            )
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = log.level.label(),
                        style = MaterialTheme.typography.labelMedium,
                        color = visuals.accent,
                        fontWeight = FontWeight.Bold
                    )
                    if (log.tag != null) {
                        Spacer(modifier = Modifier.width(8.dp))
                        Surface(
                            color = MaterialTheme.colorScheme.surface.copy(alpha = 0.75f),
                            shape = MaterialTheme.shapes.small
                        ) {
                            Text(
                                text = log.tag,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
                Text(
                    text = log.message,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                if (log.timestamp != null) {
                    Text(
                        text = log.timestamp,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

private enum class LogLevelFilter(val label: String) {
    All("全部"),
    Info("信息"),
    Error("错误"),
    Debug("调试"),
    Other("其他");

    fun matches(level: RuntimeLogLevel): Boolean {
        return when (this) {
            All -> true
            Info -> level == RuntimeLogLevel.Info
            Error -> level == RuntimeLogLevel.Error
            Debug -> level == RuntimeLogLevel.Debug
            Other -> level == RuntimeLogLevel.Other
        }
    }
}

private data class LogVisuals(
    val icon: ImageVector,
    val accent: Color,
    val background: Color
)

private fun RuntimeLogLevel.label(): String {
    return when (this) {
        RuntimeLogLevel.Info -> LogLevelFilter.Info.label
        RuntimeLogLevel.Error -> LogLevelFilter.Error.label
        RuntimeLogLevel.Debug -> LogLevelFilter.Debug.label
        RuntimeLogLevel.Other -> LogLevelFilter.Other.label
    }
}

private fun RuntimeLogLevel.visuals(): LogVisuals {
    return when (this) {
        RuntimeLogLevel.Error -> LogVisuals(Icons.Default.Warning, Color(0xFFB3261E), Color(0xFFFFF1F1))
        RuntimeLogLevel.Debug -> LogVisuals(Icons.Default.BugReport, Color(0xFF5D6B82), Color(0xFFF3F5F8))
        RuntimeLogLevel.Info -> LogVisuals(Icons.Default.Info, Color(0xFF17663A), Color(0xFFF1FAF4))
        RuntimeLogLevel.Other -> LogVisuals(Icons.Default.Info, Color(0xFF355070), Color(0xFFF7F7FA))
    }
}
