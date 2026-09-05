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
import androidx.compose.foundation.lazy.LazyColumn
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
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.components.RadioOptionRow
import com.moi.lumine.ui.components.SectionHeader

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
                        Text(
                            text = "规则路径",
                            style = MaterialTheme.typography.labelLarge,
                            color = MaterialTheme.colorScheme.primary
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text = ruleKey,
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
            }

            item {
                SectionHeader(
                    text = "代理模式 (Mode)",
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
                )
            }

            val modes = listOf("tls-rf", "raw", "direct", "block", "ttl-d")
            items(modes.size, key = { modes[it] }) { index ->
                val m = modes[index]
                RadioOptionRow(
                    label = m,
                    selected = (mode == m),
                    onClick = { mode = m }
                )
            }

            item {
                Spacer(modifier = Modifier.height(8.dp))
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
            }
        }
    }
}
