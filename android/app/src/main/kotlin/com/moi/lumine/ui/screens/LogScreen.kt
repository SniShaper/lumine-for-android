@file:OptIn(androidx.compose.foundation.layout.ExperimentalLayoutApi::class)

package com.moi.lumine.ui.screens

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
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.BugReport
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
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
import androidx.compose.material3.Surface
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun LogScreen(navController: NavController, viewModel: ConfigViewModel) {
    val logs by viewModel.logs.collectAsState()
    val vpnStatus by viewModel.vpnStatus.collectAsState()
    val listState = rememberLazyListState()
    var isAutoScrollEnabled by remember { mutableStateOf(true) }
    var selectedLevel by remember { mutableStateOf(LogLevelFilter.All) }
    val parsedLogs = remember(logs) { logs.map(::toLogEntry) }
    val filteredLogs = remember(parsedLogs, selectedLevel) {
        if (selectedLevel == LogLevelFilter.All) parsedLogs
        else parsedLogs.filter { it.level == selectedLevel }
    }
    val errorCount = remember(parsedLogs) { parsedLogs.count { it.level == LogLevelFilter.Error } }
    val infoCount = remember(parsedLogs) { parsedLogs.count { it.level == LogLevelFilter.Info } }
    val debugCount = remember(parsedLogs) { parsedLogs.count { it.level == LogLevelFilter.Debug } }

    if (isAutoScrollEnabled && filteredLogs.isNotEmpty()) {
        LaunchedEffect(filteredLogs.size, isAutoScrollEnabled, selectedLevel) {
            listState.animateScrollToItem(filteredLogs.lastIndex)
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
                    IconButton(onClick = { isAutoScrollEnabled = !isAutoScrollEnabled }) {
                        Icon(
                            if (isAutoScrollEnabled) Icons.Default.Pause else Icons.Default.PlayArrow,
                            contentDescription = "Toggle Auto-scroll"
                        )
                    }
                    IconButton(onClick = { viewModel.clearLogs() }) {
                        Icon(Icons.Default.DeleteSweep, contentDescription = "Clear Logs")
                    }
                }
            )
        }
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
                    totalCount = parsedLogs.size,
                    infoCount = infoCount,
                    errorCount = errorCount,
                    debugCount = debugCount
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
                        hasAnyLogs = parsedLogs.isNotEmpty(),
                        isRunning = vpnStatus.phase == "running" || vpnStatus.phase == "starting"
                    )
                }
            } else {
                itemsIndexed(filteredLogs, key = { index, item -> "${item.raw}-$index" }) { _, log ->
                    LogItem(log)
                }
            }
        }
    }
}

@Composable
private fun LogSummaryCard(
    statusMessage: String,
    totalCount: Int,
    infoCount: Int,
    errorCount: Int,
    debugCount: Int
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
                text = if (hasAnyLogs) "切换日志级别后再看一次。" else "启动后这里会展示代理核心的真实运行信息。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun LogItem(log: LogEntry) {
    Card(
        colors = CardDefaults.cardColors(containerColor = log.background)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Icon(
                imageVector = log.icon,
                contentDescription = null,
                tint = log.accent,
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
                        text = log.level.label,
                        style = MaterialTheme.typography.labelMedium,
                        color = log.accent,
                        fontWeight = FontWeight.Bold
                    )
                    if (log.tag != null) {
                        Spacer(modifier = Modifier.width(8.dp))
                        AssistChip(
                            onClick = {},
                            enabled = false,
                            label = {
                                Text(
                                    text = log.tag,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        )
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
    Other("其他")
}

private data class LogEntry(
    val raw: String,
    val timestamp: String?,
    val tag: String?,
    val level: LogLevelFilter,
    val message: String,
    val icon: ImageVector,
    val accent: Color,
    val background: Color
)

private fun toLogEntry(raw: String): LogEntry {
    val timestampMatch = Regex("""^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} """).find(raw)
    val timestamp = timestampMatch?.value?.trim()
    val withoutTime = if (timestamp != null) raw.removePrefix(timestampMatch.value).trim() else raw.trim()

    val tagMatch = Regex("""^\[[^\]]+\]""").find(withoutTime)
    val tag = tagMatch?.value
    val remainder = if (tag != null) withoutTime.removePrefix(tag).trim() else withoutTime

    val level = when {
        remainder.contains("ERROR", ignoreCase = true) || remainder.contains("failed", ignoreCase = true) -> LogLevelFilter.Error
        remainder.contains("DEBUG", ignoreCase = true) || remainder.contains("closed", ignoreCase = true) -> LogLevelFilter.Debug
        remainder.contains("INFO", ignoreCase = true) || remainder.contains("started", ignoreCase = true) || remainder.contains("CONNECT") -> LogLevelFilter.Info
        else -> LogLevelFilter.Other
    }

    val (icon, accent, background) = when (level) {
        LogLevelFilter.Error -> Triple(Icons.Default.Warning, Color(0xFFB3261E), Color(0xFFFFF1F1))
        LogLevelFilter.Debug -> Triple(Icons.Default.BugReport, Color(0xFF5D6B82), Color(0xFFF3F5F8))
        LogLevelFilter.Info -> Triple(Icons.Default.Info, Color(0xFF17663A), Color(0xFFF1FAF4))
        LogLevelFilter.All, LogLevelFilter.Other -> Triple(Icons.Default.Info, Color(0xFF355070), Color(0xFFF7F7FA))
    }

    return LogEntry(
        raw = raw,
        timestamp = timestamp,
        tag = tag,
        level = level,
        message = remainder,
        icon = icon,
        accent = accent,
        background = background
    )
}
