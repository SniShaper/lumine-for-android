package com.moi.lumine.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen

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
                    .padding(16.dp),
                placeholder = { Text("搜索域名或 IP 规则...") },
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                singleLine = true,
                colors = TextFieldDefaults.textFieldColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.3f)
                )
            )

            LazyColumn(modifier = Modifier.weight(1f)) {
                item {
                    Text(
                        "域名规则 (${domainRules.size})",
                        style = MaterialTheme.typography.titleSmall,
                        modifier = Modifier.padding(16.dp),
                        color = MaterialTheme.colorScheme.primary
                    )
                }
                items(items = domainRules, key = { it }) { key ->
                    RuleItem(key, "domain") {
                        viewModel.setEditingRule(key)
                        navController.navigate(Screen.RuleDetail.createRoute("domain"))
                    }
                }

                item {
                    Text(
                        "IP 规则 (${ipRules.size})",
                        style = MaterialTheme.typography.titleSmall,
                        modifier = Modifier.padding(16.dp),
                        color = MaterialTheme.colorScheme.primary
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
                Text("规则类型", style = MaterialTheme.typography.labelLarge)
                RuleTypeOption(
                    label = "域名规则",
                    selected = selectedType == "domain",
                    onClick = { selectedType = "domain" }
                )
                RuleTypeOption(
                    label = "IP 规则",
                    selected = selectedType == "ip",
                    onClick = { selectedType = "ip" }
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
private fun RuleTypeOption(label: String, selected: Boolean, onClick: () -> Unit) {
    androidx.compose.foundation.layout.Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically
    ) {
        RadioButton(selected = selected, onClick = onClick)
        Text(label, modifier = Modifier.padding(start = 8.dp))
    }
}

@Composable
fun RuleItem(key: String, type: String, onClick: () -> Unit) {
    androidx.compose.material3.ListItem(
        headlineContent = { Text(key, maxLines = 1) },
        supportingContent = { Text(if (type == "domain") "Domain Pattern" else "IP CIDR") },
        modifier = Modifier.clickable { onClick() }
    )
    HorizontalDivider(
        modifier = Modifier.padding(horizontal = 16.dp),
        thickness = 0.5.dp,
        color = MaterialTheme.colorScheme.outlineVariant
    )
}
