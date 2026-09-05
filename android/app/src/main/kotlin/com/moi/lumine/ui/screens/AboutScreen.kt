package com.moi.lumine.ui.screens

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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Assignment
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Security
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.moi.lumine.R
import com.moi.lumine.ui.components.SectionHeader

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AboutScreen(navController: NavController) {
    val context = LocalContext.current
    val packageInfo = remember {
        runCatching {
            context.packageManager.getPackageInfo(context.packageName, 0)
        }.getOrNull()
    }
    val versionName = packageInfo?.versionName ?: "unknown"

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("关于") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { innerPadding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            item {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraLarge,
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
                        contentColor = MaterialTheme.colorScheme.onSurface
                    ),
                    elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(20.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Surface(
                            modifier = Modifier.size(72.dp),
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.secondaryContainer
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                // 使用矢量前景自绘品牌标记；R.mipmap.ic_launcher 在 API 26+
                                // 是 AdaptiveIcon，painterResource 不支持，会抛异常。
                                Icon(
                                    painter = painterResource(id = R.drawable.ic_launcher_foreground),
                                    contentDescription = null,
                                    modifier = Modifier.size(44.dp),
                                    tint = MaterialTheme.colorScheme.onSecondaryContainer
                                )
                            }
                        }
                        Spacer(modifier = Modifier.width(16.dp))
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "Lumine",
                                style = MaterialTheme.typography.headlineSmall,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                            Text(
                                text = "本地代理 · 轻量 VPN 客户端",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
            }

            item {
                AboutSection(text = "版本信息")
                AboutParagraph(
                    text = "当前版本 $versionName，支持 Android 7.0（API 24）及以上系统。"
                )
            }

            item {
                AboutSection(text = "简介")
                AboutParagraph(
                    text = "Lumine 是基于 enimul Go 核心的 Android 端 Clash 风格本地代理 / VPN 客户端。" +
                        "它通过 Android VPNService（TUN）隧道接管设备流量，并按配置规则在本地完成转发与分流。"
                )
                AboutParagraph(
                    text = "界面采用 Kotlin + Jetpack Compose 原生构建，遵循 Material Design 3 规范并支持" +
                        "动态取色（Android 12+）；Go 核心经 gomobile 编译为单个 AAR 接入，无任何 WebView 内嵌。"
                )
                AboutParagraph(
                    text = "本项目是 SniShaper 代理项目的移动端配套版本：SniShaper 提供 Windows / Linux" +
                        "桌面端与 headless CLI，Lumine 面向 Android 移动场景。"
                )
            }

            item {
                SectionHeader(
                    text = "功能",
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
                    Column {
                        AboutFeatureRow(Icons.Default.PowerSettingsNew, "一键本地代理（TUN）", "首页开关即可启停 VPNService 隧道")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Refresh, "订阅管理", "URL 导入、刷新并切换多套 Clash 风格配置")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Tune, "规则引擎", "查看、新建与编辑域名和 IP/CIDR 规则，支持多种代理模式")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Dns, "智能分流", "继承 enimul 的 GFWList 黑名单驱动分流，配合灵活 Fake-IP")
                        AboutDivider()
                        AboutFeatureRow(Icons.AutoMirrored.Filled.Assignment, "实时日志", "级别过滤、自动滚动、捕捉开关与一键导出")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Settings, "全局设置", "上游 DNS 地址与核心日志级别")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Security, "后台保活", "无障碍服务、自启动、电池优化等引导设置")
                    }
                }
            }

            item {
                SectionHeader(
                    text = "技术栈",
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
                )
                AboutParagraph(
                    text = "界面：Kotlin + Jetpack Compose，遵循 Material Design 3 形状、排版与配色规范，" +
                        "Android 12+ 跟随系统壁纸动态取色，低版本回退基线配色，深浅色双支持。"
                )
                AboutParagraph(
                    text = "核心：Go（enimul 代理与分流内核），经 gomobile 绑定为单个 AAR（LumineCore.aar）接入。" +
                        "隧道基于 Android VPNService 实现，无 WebView 内嵌。"
                )
            }

            item {
                SectionHeader(
                    text = "获取渠道",
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
                )
                AboutParagraph(
                    text = "GitHub Releases：按设备 ABI 提供 arm64-v8a / armeabi-v7a / x86_64 / x86 分包，推荐 arm64-v8a。"
                )
                AboutParagraph(
                    text = "F-Droid：包名 com.moi.lumine，可在 F-Droid 商店直接搜索安装。"
                )
            }

            item {
                SectionHeader(
                    text = "致谢",
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
                    Column {
                        AboutFeatureRow(Icons.Default.Description, "enimul", "Go 代理与分流核心（上游）")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Info, "lumine", "enimul 的前身项目")
                        AboutDivider()
                        AboutFeatureRow(Icons.Default.Security, "SniShaper", "桌面版代理项目，路由理念同源")
                    }
                }
            }

            item {
                SectionHeader(
                    text = "开源许可",
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
                )
                AboutParagraph(text = "本项目基于 GNU Affero General Public License v3.0（AGPL-3.0）协议开源发布。")
            }
        }
    }
}

@Composable
private fun AboutSection(text: String) {
    SectionHeader(text = text, modifier = Modifier.padding(top = 8.dp, bottom = 4.dp))
}

@Composable
private fun AboutParagraph(text: String) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.extraLarge,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
            contentColor = MaterialTheme.colorScheme.onSurface
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(16.dp)
        )
    }
}

@Composable
private fun AboutFeatureRow(icon: ImageVector, title: String, subtitle: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Surface(
            modifier = Modifier.size(36.dp),
            shape = CircleShape,
            color = MaterialTheme.colorScheme.secondaryContainer
        ) {
            Box(contentAlignment = Alignment.Center) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.onSecondaryContainer
                )
            }
        }
        Spacer(modifier = Modifier.width(16.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(2.dp))
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun AboutDivider() {
    HorizontalDivider(
        modifier = Modifier.padding(start = 68.dp),
        color = MaterialTheme.colorScheme.outlineVariant
    )
}
