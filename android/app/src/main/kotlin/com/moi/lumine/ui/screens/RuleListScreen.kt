package com.moi.lumine.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RuleListScreen(navController: NavController, viewModel: ConfigViewModel) {
    val config by viewModel.currentConfig.collectAsState()
    val selectedConfig by viewModel.selectedConfigDisplayName.collectAsState()
    var searchText by remember { mutableStateOf("") }
    var showCreateDialog by remember { mutableStateOf(false) }

    val domainRules = remember(config.domainPolicies, searchText) {
        config.domainPolicies.keys
            .filter { it.contains(searchText, ignoreCase = true) }
            .sorted()
    }
    val ipRules = remember(config.ipPolicies, searchText) {
        config.ipPolicies.keys
            .filter { it.contains(searchText, ignoreCase = true) }
            .sorted()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text("规则")
                        Text(
                            text = selectedConfig,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = { showCreateDialog = true }) {
                        Icon(Icons.Default.Add, contentDescription = "New Rule")
                    }
                }
            )
        }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
        ) {
            TextField(
                value = searchText,
                onValueChange = { searchText = it },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 8.dp),
                placeholder = { Text("搜索域名或 IP 规则...") },
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                singleLine = true,
                shape = MaterialTheme.shapes.large
            )

            LazyColumn(modifier = Modifier.weight(1f)) {
                if (domainRules.isEmpty() && ipRules.isEmpty()) {
                    item {
                        Text(
                            text = "没有匹配的规则",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
                item {
                    SectionHeader(
                        text = "域名规则 (${domainRules.size})",
                        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
                    )
                }
                items(items = domainRules, key = { it }) { key ->
                    RuleItem(key, "domain") {
                        viewModel.setEditingRule(key)
                        navController.navigate(Screen.RuleDetail.createRoute("domain"))
                    }
                }

                item {
                    SectionHeader(
                        text = "IP 规则 (${ipRules.size})",
                        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = 4.dp)
                    )
                }
                items(items = ipRules, key = { it }) { key ->
                    RuleItem(key, "ip") {
                        viewModel.setEditingRule(key)
                        navController.navigate(Screen.RuleDetail.createRoute("ip"))
                    }
                }
            }
        }
    }

    if (showCreateDialog) {
        CreateRuleDialog(
            onDismiss = { showCreateDialog = false },
            onConfirm = { type, key ->
                viewModel.setEditingRule(key)
                showCreateDialog = false
                navController.navigate(Screen.RuleDetail.createRoute(type))
            }
        )
    }
}

@Composable
private fun CreateRuleDialog(
    onDismiss: () -> Unit,
    onConfirm: (type: String, key: String) -> Unit
) {
    var selectedType by remember { mutableStateOf("domain") }
    var ruleKey by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("新建规则") },
        text = {
            Column {
                RadioOptionRow(
                    label = "域名规则",
                    selected = selectedType == "domain",
                    onClick = { selectedType = "domain" },
                    description = "按域名匹配，例如 *.bing.com"
                )
                RadioOptionRow(
                    label = "IP 规则",
                    selected = selectedType == "ip",
                    onClick = { selectedType = "ip" },
                    description = "按 IP/CIDR 匹配，例如 1.2.3.0/24"
                )
                TextField(
                    value = ruleKey,
                    onValueChange = { ruleKey = it },
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                    label = { Text(if (selectedType == "domain") "域名匹配" else "IP/CIDR") },
                    placeholder = { Text(if (selectedType == "domain") "例如 *.bing.com" else "例如 1.2.3.0/24") },
                    singleLine = true
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { onConfirm(selectedType, ruleKey.trim()) },
                enabled = ruleKey.isNotBlank()
            ) {
                Text("继续")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("取消")
            }
        }
    )
}

@Composable
fun RuleItem(key: String, type: String, onClick: () -> Unit) {
    val isDomain = type == "domain"
    ListItem(
        headlineContent = {
            Text(
                text = key,
                maxLines = 1,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
        },
        supportingContent = {
            Text(
                text = if (isDomain) "域名规则" else "IP/CIDR",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        },
        leadingContent = {
            Surface(
                modifier = Modifier.size(40.dp),
                shape = CircleShape,
                color = MaterialTheme.colorScheme.secondaryContainer
            ) {
                Row(
                    modifier = Modifier.fillMaxSize(),
                    horizontalArrangement = androidx.compose.foundation.layout.Arrangement.Center,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Icon(
                        imageVector = if (isDomain) Icons.Default.Language else Icons.Default.Dns,
                        contentDescription = null,
                        modifier = Modifier.size(20.dp),
                        tint = MaterialTheme.colorScheme.onSecondaryContainer
                    )
                }
            }
        },
        trailingContent = {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        },
        colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surface),
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() }
    )
    HorizontalDivider(
        modifier = Modifier.padding(start = 72.dp, end = 16.dp),
        color = MaterialTheme.colorScheme.outlineVariant
    )
}
