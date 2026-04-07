package com.moi.lumine.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.model.SubscriptionProfile
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.SubscriptionImportStage
import com.moi.lumine.ui.SubscriptionImportState
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import kotlinx.coroutines.delay

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SubscriptionScreen(navController: NavController, viewModel: ConfigViewModel) {
    val subscriptions by viewModel.subscriptions.collectAsState()
    val selectedConfigName by viewModel.selectedConfigName.collectAsState()
    val busyId by viewModel.subscriptionBusyId.collectAsState()
    val isRefreshingAll by viewModel.isRefreshingAllSubscriptions.collectAsState()
    val message by viewModel.subscriptionMessage.collectAsState()
    val importState by viewModel.subscriptionImportState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }
    var showAddDialog by remember { mutableStateOf(false) }

    LaunchedEffect(message) {
        val current = message ?: return@LaunchedEffect
        snackbarHostState.showSnackbar(current)
        viewModel.consumeSubscriptionMessage()
    }

    LaunchedEffect(showAddDialog, importState.stage) {
        if (!showAddDialog || importState.stage != SubscriptionImportStage.Success) {
            return@LaunchedEffect
        }
        delay(850)
        showAddDialog = false
        viewModel.resetSubscriptionImportState()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("配置订阅") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(
                        onClick = { viewModel.refreshAllSubscriptions() },
                        enabled = subscriptions.isNotEmpty() && !isRefreshingAll
                    ) {
                        if (isRefreshingAll) {
                            CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                        } else {
                            Icon(Icons.Default.Refresh, contentDescription = "Refresh All")
                        }
                    }
                    IconButton(onClick = { showAddDialog = true }) {
                        Icon(Icons.Default.Add, contentDescription = "Add Subscription")
                    }
                }
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { innerPadding ->
        if (subscriptions.isEmpty()) {
            EmptySubscriptionState(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                onAdd = { showAddDialog = true }
            )
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                items(items = subscriptions, key = { it.id }) { subscription ->
                    SubscriptionCard(
                        subscription = subscription,
                        selected = selectedConfigName == subscription.configName,
                        isBusy = busyId == subscription.id,
                        onApply = { viewModel.applySubscription(subscription) },
                        onRefresh = { viewModel.refreshSubscription(subscription) },
                        onDelete = { viewModel.deleteSubscription(subscription) }
                    )
                }
            }
        }
    }

    if (showAddDialog) {
        AddSubscriptionDialog(
            importState = importState,
            isBusy = busyId == ConfigViewModel.NEW_SUBSCRIPTION_ID,
            onDismiss = {
                if (busyId != ConfigViewModel.NEW_SUBSCRIPTION_ID) {
                    showAddDialog = false
                    viewModel.resetSubscriptionImportState()
                }
            },
            onConfirm = { name, url ->
                viewModel.addSubscription(name, url)
            }
        )
    }
}

@Composable
private fun EmptySubscriptionState(modifier: Modifier = Modifier, onAdd: () -> Unit) {
    Box(modifier = modifier.padding(16.dp), contentAlignment = Alignment.Center) {
        Card(
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
        ) {
            Column(
                modifier = Modifier.padding(20.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text("还没有配置订阅", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                Text(
                    "添加一个 URL 订阅后，就可以像 Clash 一样在多个配置之间切换。",
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Button(onClick = onAdd) {
                    Text("添加订阅")
                }
            }
        }
    }
}

@Composable
private fun SubscriptionCard(
    subscription: SubscriptionProfile,
    selected: Boolean,
    isBusy: Boolean,
    onApply: () -> Unit,
    onRefresh: () -> Unit,
    onDelete: () -> Unit
) {
    var menuExpanded by remember { mutableStateOf(false) }
    val containerColor by animateColorAsState(
        targetValue = when {
            selected -> MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.72f)
            isBusy -> MaterialTheme.colorScheme.surfaceVariant
            else -> MaterialTheme.colorScheme.surface
        },
        animationSpec = tween(durationMillis = 220),
        label = "subscription_container"
    )

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .animateContentSize(animationSpec = tween(durationMillis = 220))
            .clickable(enabled = !isBusy, onClick = onApply),
        colors = CardDefaults.cardColors(containerColor = containerColor)
    ) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                RadioButton(selected = selected, onClick = onApply, enabled = !isBusy)
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .padding(start = 8.dp)
                ) {
                    Text(
                        text = subscription.name,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.SemiBold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = subscription.url,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "${formatRelativeTime(subscription.updatedAt)}  ·  ${formatAbsoluteTime(subscription.updatedAt)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "配置名：${subscription.configName}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }

                Box {
                    IconButton(onClick = { menuExpanded = true }) {
                        if (isBusy) {
                            CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp)
                        } else {
                            Icon(Icons.Default.MoreVert, contentDescription = "More")
                        }
                    }
                    DropdownMenu(expanded = menuExpanded, onDismissRequest = { menuExpanded = false }) {
                        DropdownMenuItem(
                            text = { Text(if (selected) "当前已应用" else "应用此配置") },
                            onClick = {
                                menuExpanded = false
                                if (!selected) onApply()
                            }
                        )
                        DropdownMenuItem(
                            text = { Text("刷新订阅") },
                            onClick = {
                                menuExpanded = false
                                onRefresh()
                            }
                        )
                        DropdownMenuItem(
                            text = { Text("删除订阅") },
                            onClick = {
                                menuExpanded = false
                                onDelete()
                            }
                        )
                    }
                }
            }
            AnimatedVisibility(
                visible = isBusy,
                enter = fadeIn(animationSpec = tween(180)) + expandVertically(animationSpec = tween(180)),
                exit = fadeOut(animationSpec = tween(180)) + shrinkVertically(animationSpec = tween(180))
            ) {
                LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            }
            if (!isBusy) {
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            }
        }
    }
}

@Composable
private fun AddSubscriptionDialog(
    importState: SubscriptionImportState,
    isBusy: Boolean,
    onDismiss: () -> Unit,
    onConfirm: (name: String, url: String) -> Unit
) {
    var name by remember { mutableStateOf("") }
    var url by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (importState.stage == SubscriptionImportStage.Success) "导入完成" else "添加订阅") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("配置名称") },
                    singleLine = true,
                    enabled = !isBusy
                )
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text("订阅 URL") },
                    singleLine = true,
                    enabled = !isBusy
                )

                AnimatedVisibility(
                    visible = importState.stage != SubscriptionImportStage.Idle,
                    enter = fadeIn(animationSpec = tween(180)) + expandVertically(animationSpec = tween(180)),
                    exit = fadeOut(animationSpec = tween(180)) + shrinkVertically(animationSpec = tween(180))
                ) {
                    Card(
                        colors = CardDefaults.cardColors(
                            containerColor = when (importState.stage) {
                                SubscriptionImportStage.Success -> MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.88f)
                                SubscriptionImportStage.Error -> MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.9f)
                                else -> MaterialTheme.colorScheme.surfaceVariant
                            }
                        )
                    ) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(14.dp),
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            Text(
                                text = importState.title,
                                style = MaterialTheme.typography.titleSmall,
                                fontWeight = FontWeight.SemiBold
                            )
                            importState.detail?.takeIf { it.isNotBlank() }?.let { detail ->
                                Text(
                                    text = detail,
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                            when {
                                importState.progress != null -> {
                                    LinearProgressIndicator(
                                        progress = { importState.progress.coerceIn(0f, 1f) },
                                        modifier = Modifier.fillMaxWidth()
                                    )
                                }
                                importState.stage != SubscriptionImportStage.Error -> {
                                    LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                                }
                            }
                            if (importState.stage == SubscriptionImportStage.Success) {
                                Text(
                                    text = "配置即将出现在列表中",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = { onConfirm(name, url) },
                enabled = !isBusy
            ) {
                Text(
                    when {
                        isBusy -> "导入中..."
                        importState.stage == SubscriptionImportStage.Error -> "重试"
                        else -> "导入"
                    }
                )
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss, enabled = !isBusy) {
                Text(if (importState.stage == SubscriptionImportStage.Success) "完成" else "取消")
            }
        }
    )
}

private fun formatRelativeTime(timestamp: Long): String {
    if (timestamp <= 0L) {
        return "尚未更新"
    }
    val delta = (System.currentTimeMillis() - timestamp).coerceAtLeast(0L)
    val minutes = delta / 60_000L
    val hours = delta / 3_600_000L
    val days = delta / 86_400_000L
    return when {
        minutes < 1L -> "刚刚"
        minutes < 60L -> "$minutes 分钟前"
        hours < 24L -> "$hours 小时前"
        else -> "$days 天前"
    }
}

private fun formatAbsoluteTime(timestamp: Long): String {
    if (timestamp <= 0L) {
        return "--"
    }
    val formatter = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
    return formatter.format(Date(timestamp))
}
