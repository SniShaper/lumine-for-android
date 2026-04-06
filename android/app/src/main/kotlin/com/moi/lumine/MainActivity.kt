package com.moi.lumine

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavType
import androidx.navigation.compose.*
import androidx.navigation.navArgument
import com.moi.lumine.ui.ConfigViewModel
import com.moi.lumine.ui.Screen
import com.moi.lumine.ui.screens.*
import com.moi.lumine.ui.theme.LumineTheme
import mobile.Mobile

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        window.statusBarColor = android.graphics.Color.TRANSPARENT
        window.navigationBarColor = android.graphics.Color.TRANSPARENT
        WindowInsetsControllerCompat(window, window.decorView).apply {
            isAppearanceLightStatusBars = true
            isAppearanceLightNavigationBars = true
        }
        setContent {
            LumineTheme {
                MainContainer()
            }
        }
    }
}

@Composable
fun MainContainer() {
    val context = LocalContext.current
    val navController = rememberNavController()
    val viewModel: ConfigViewModel = viewModel()
    
    val vpnRequestLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == ComponentActivity.RESULT_OK) {
            VpnRuntimeState.setStatus("starting", "权限已授予，正在启动服务")
            val intent = Intent(context, LumineVpnService::class.java).apply {
                putExtra("CONFIG_NAME", viewModel.selectedConfigName.value)
            }
            ContextCompat.startForegroundService(context, intent)
        } else {
            VpnRuntimeState.setStatus("idle", "VPN 权限未授予")
        }
    }

    fun startVpn() {
        val vpnIntent = VpnService.prepare(context)
        if (vpnIntent != null) {
            VpnRuntimeState.setStatus("authorizing", "等待 VPN 权限授权")
            vpnRequestLauncher.launch(vpnIntent)
        } else {
            VpnRuntimeState.setStatus("starting", "正在启动服务")
            val intent = Intent(context, LumineVpnService::class.java).apply {
                putExtra("CONFIG_NAME", viewModel.selectedConfigName.value)
            }
            ContextCompat.startForegroundService(context, intent)
        }
    }

    fun stopVpn() {
        val intent = Intent(context, LumineVpnService::class.java).apply {
            action = "STOP"
        }
        context.startService(intent)
        viewModel.setVpnActive(false)
    }

    Scaffold { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = Screen.Home.route,
            modifier = Modifier.padding(innerPadding)
        ) {
            composable(Screen.Home.route) {
                HomeScreen(
                    navController = navController,
                    viewModel = viewModel,
                    onStart = { startVpn() },
                    onStop = { stopVpn() }
                )
            }
            composable(Screen.Subscriptions.route) { SubscriptionScreen(navController, viewModel) }
            composable(Screen.Rules.route) { RuleListScreen(navController, viewModel) }
            composable(
                Screen.RuleDetail.route,
                arguments = listOf(
                    navArgument("type") { type = NavType.StringType }
                )
            ) { backStackEntry ->
                val type = backStackEntry.arguments?.getString("type") ?: ""
                RuleEditorScreen(navController, viewModel, type)
            }
            composable(Screen.Settings.route) { GlobalSettingsScreen(navController, viewModel) }
            composable(Screen.Logs.route) { LogScreen(navController, viewModel) }
        }
    }
}


@Composable
fun TestScreen(onStart: () -> Unit) {
    var version by remember { mutableStateOf("Loading...") }
    val spliceMsg = remember { Mobile.helloSplice() }

    LaunchedEffect(Unit) {
        version = Mobile.getVersion()
    }

    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(text = "Lumine Mobile", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(8.dp))
        Text(text = "Version: $version")
        Text(text = "Syscall Test: $spliceMsg")
        Spacer(modifier = Modifier.height(16.dp))
        Button(onClick = onStart) {
            Text("Start VPN")
        }
    }
}
