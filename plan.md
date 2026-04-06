IMPLEMENTATION_PLAN.MD
Lumine Android 移植最终实施方案
经过多轮沟通与技术调研，本项目将按以下确定的技术路线进行移植开发。

1. 技术架构 (Technical Architecture)
核心网络引擎 (Go Core):
DPI 规避逻辑：完全复用原版 internal/desync_linux.go 中的 splice/vmsplice 逻辑。经验证，该逻辑在 Android (Termux) 环境下通过 seccomp 检查且可行。
协议支持：维持原版 SOCKS5/HTTP 代理功能，并集成 tun2socks 以支持 Android VpnService 系统级代理。
配置系统：维持原版 JSON 格式 配置文件，无需兼容 Clash YAML。
Android 前端 (Frontend):
技术栈：Kotlin + Jetpack Compose。
设计语言：参考 Clash Verge 的现代设计风格（简洁、流式布局、深色模式优化）。
进程管理:
VpnService: 利用 Android 系统 Service 确保核心在后台稳定运行。
AAR 封装: 使用 gomobile 将 Go 核心打包为 Android 库。
2. 功能规划 (Feature Roadmap)
第一阶段：基础架构与内核打通
 使用 gomobile 封装 Lumine 核心为 AAR 库。
 实现 Android VpnService 基础框架，集成 tun2socks 转发链路。
 验证系统级代理下的 DPI 规避（tls-rf 和 ttl-d）有效性。
第二阶段：仿 Clash Verge 界面开发
 仪表盘 (Dashboard)：实现实时上传/下载速度统计、活跃连接列表。
 节点管理 (Proxies)：实现基于原版配置的分组展示与连通性测试（Ping/延迟）。
 配置中心 (Configs)：支持多份 JSON 配置文件管理、本地导入与简单的文本编辑。
 日志监控 (Logs)：实时流式输出 Go 核心日志。
第三阶段：系统功能与优化
 分应用代理：允许用户选择哪些 App 绕过 VPN。
 自动启停：支持开机自启、通知栏快捷开关。
 稳定性优化：解决应用在后台可能出现的异常中断问题。
3. 验证计划 (Verification Plan)
连通性测试：确保常规网页访问正常。
规避效果测试：在受限网络环境下验证 tls-rf 和 ttl-d 是否能成功穿透检测。
性能评估：测试在高带宽下载时的 CPU 占用与耗电量。
TASK.MD
 Phase 1: 核心引擎与 VPN 链路封装 (Go + AAR)
 整合 Lumine 核心代码与 tun2socks 转发逻辑
 配置 gomobile 环境，生成 LumineCore.aar
 编写简单的测试 Activity，验证 AAR 的启动与停止
 验证 vmsplice 和 splice 系统调用的实机表现（DPI 规避效果）
 Phase 2: Android VpnService 系统集成 (Kotlin)
 实现基础 LumineVpnService 类，处理 VPN 启动与权限请求
 实现后台进程通信（IPC），同步 VPN 状态至 UI
 完善前台通知栏，提供快捷状态切与性能概览
 Phase 3: 仿 Clash Verge UI 界面实现 (Jetpack Compose)
 Dashboard 页面：流量实时统计图表、延迟测速看板
 Proxies 分组页面：嵌套分组列表、并行的连通性测试（Ping）
 Configs 配置文件页面：多 JSON 配置切换、原版 JSON 格式编辑与保存
 Logs 日志页面：集成实时 Logcat/Go-Engine 日志查看器
 Phase 4: 全局功能完善与性能优化
 实现分应用代理选择功能（Per-App Split Tunneling）
 优化 Android 系统对 VPN 会话的保活处理（Foreground Service 级别）
 导出与导入完整配置文件 JSON 包
 Phase 5: 最终验收与 Bug 修复
 修复多级代理下可能的连接超时问题
 对齐 Clash Verge 的色彩方案与深色模式效果