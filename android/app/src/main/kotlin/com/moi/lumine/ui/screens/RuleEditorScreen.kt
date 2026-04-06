package com.moi.lumine.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.model.Policy

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RuleEditorScreen(navController: NavController, viewModel: ConfigViewModel, type: String) {
    val config by viewModel.currentConfig.collectAsState()
    val key by viewModel.editingRuleKey.collectAsState()
    val ruleKey = key

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

    var mode by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.mode ?: "tls-rf") }
    var host by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.host ?: "") }
    var mapTo by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.mapTo ?: "") }
    var tls13Only by remember(ruleKey, initialPolicy) { mutableStateOf(initialPolicy.tls13Only ?: false) }

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
                        viewModel.setEditingRule(null)
                        navController.popBackStack()
                    }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    IconButton(onClick = {
                        val updatedPolicy = initialPolicy.copy(
                            mode = mode,
                            host = host.ifEmpty { null },
                            mapTo = mapTo.ifEmpty { null },
                            tls13Only = tls13Only
                        )
                        val updatedConfig = if (type == "domain") {
                            config.copy(domainPolicies = config.domainPolicies + (ruleKey to updatedPolicy))
                        } else {
                            config.copy(ipPolicies = config.ipPolicies + (ruleKey to updatedPolicy))
                        }
                        viewModel.updateConfig(updatedConfig)
                        viewModel.saveConfig()
                        viewModel.setEditingRule(null)
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
            contentPadding = PaddingValues(vertical = 16.dp)
        ) {
            item {
                Text("规则路径", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
                Text(ruleKey, style = MaterialTheme.typography.bodyLarge)
                Spacer(modifier = Modifier.height(24.dp))
            }

            item {
                Text("代理模式 (Mode)", style = MaterialTheme.typography.labelLarge)
            }

            val modes = listOf("tls-rf", "raw", "direct", "block", "ttl-d")
            items(modes.size, key = { modes[it] }) { index ->
                val m = modes[index]
                Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                    RadioButton(selected = (mode == m), onClick = { mode = m })
                    Text(m, modifier = Modifier.padding(start = 8.dp))
                }
            }

            item {
                Spacer(modifier = Modifier.height(16.dp))
                OutlinedTextField(
                    value = host,
                    onValueChange = { host = it },
                    label = { Text("目标主机 (Host Overwrite)") },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("例如 1.1.1.1 或 self") }
                )
                Spacer(modifier = Modifier.height(16.dp))
                OutlinedTextField(
                    value = mapTo,
                    onValueChange = { mapTo = it },
                    label = { Text("映射到 (Map To)") },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("例如 127.0.0.1:8080") }
                )
                Spacer(modifier = Modifier.height(16.dp))
                Row(verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                    Checkbox(checked = tls13Only, onCheckedChange = { tls13Only = it })
                    Text("仅限 TLS 1.3")
                }
            }
        }
    }
}
