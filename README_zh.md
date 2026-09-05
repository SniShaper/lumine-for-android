# Lumine for Android

[English](README.md) | [简体中文](README_zh.md)

![Android](https://img.shields.io/badge/Android-7.0%2B-3DDC84?style=flat\&logo=android\&logoColor=white)

![许可证](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat\&logo=open-source-initiative\&label=许可证)

![GitHub Release](https://img.shields.io/github/v/release/SniShaper/lumine-for-android?style=flat\&logo=github\&label=版本)

![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/lumine-for-android/total?style=flat\&logo=github\&label=下载量)

![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/lumine-for-android?style=flat\&logo=git\&label=最后提交)

![持续集成](https://img.shields.io/github/actions/workflow/status/SniShaper/lumine-for-android/android-release.yml?style=flat\&logo=githubactions\&label=CI)

**Lumine** 是基于 [enimul](https://github.com/lzpls/enimul) Go 核心（前 [lumine](https://codeberg.org/PonyCW26/lumine)）的 Android 端 Clash 风格本地代理 / VPN 客户端。它通过 Android `VPNService`（TUN）隧道接管设备流量，并按配置规则在本地完成转发与分流。

界面采用 **Kotlin + Jetpack Compose** 原生构建，遵循 **Material Design 3** 规范并支持动态取色；Go 核心经 `gomobile` 编译为单个 AAR 接入，无任何 WebView 内嵌。本项目是 [SniShaper](https://github.com/SnishaperTeam/SniShaper) 代理项目的移动端配套版本。

> 需要桌面版？参见 **[SniShaper](https://github.com/SnishaperTeam/SniShaper)** —— 基于相同路由理念的 Windows / Linux 代理客户端，另含无图形界面的 headless CLI 版本。



---

## 特性

- **一键本地代理（TUN）**：基于 Android `VPNService` 隧道，首页一个开关即可启停。
- **订阅管理**：从订阅 URL 拉取配置、手动刷新，并在多套 Clash 风格配置间随时切换。
- **规则引擎**：独立规则页面，可查看、新建与编辑 **域名** 与 **IP/CIDR** 规则，支持多种代理模式。
- **智能分流**：继承 enimul 核心的 GFWList 黑名单驱动分流，配合灵活的 Fake-IP 实现。
- **实时日志**：级别过滤（全部 / 信息 / 错误 / 调试 / 其他）、自动滚动跟随、捕捉开关与一键导出。
- **全局设置**：上游 DNS 地址与核心日志级别。
- **后台保活**：针对常见国产 ROM，提供无障碍服务、自启动、电池优化等引导设置。
- **Material Design 3 界面**：Android 12+ 跟随系统壁纸动态取色，低版本回退基线配色，深浅色双支持。

---

## 快速开始

### 从 GitHub Releases 安装

前往[最新版本](https://github.com/SniShaper/lumine-for-android/releases)下载与设备 ABI 匹配的 APK，按提示允许「安装未知来源应用」即可：

| ABI           | 常见设备                  |
| ------------- | --------------------- |
| `arm64-v8a`   | 2017 年后的主流手机 / 平板（推荐） |
| `armeabi-v7a` | 较老的 32 位设备            |
| `x86_64`      | x86_64 模拟器，如 MuMu、雷电  |
| `x86`         | 旧版 x86 模拟器            |

### 从 F-Droid 安装



包名：`com.moi.lumine`。

### 配置与启动

1. 打开应用，进入「配置订阅」。
2. 填写订阅名称与 URL，等待导入完成后点击该配置以应用。
3. 返回首页拨动开关启动代理，`VPNService` 运行后即出现隧道图标。
4. 需要更细的分流可在「规则」页面调整；国产 ROM 上若划掉应用被清理，请参考「保活设置」页完成引导。

### 模拟器 / adb 安装

```bash
adb install -r app-arm64-v8a-debug.apk   # 或与模拟器 ABI 匹配的分包
adb shell am start -n com.moi.lumine/.MainActivity
```

---

## 文档

- **[上游核心（enimul）](https://github.com/lzpls/enimul)**：了解代理模式与配置文件语法（保持与上游兼容）。
- **[桌面版（SniShaper）](https://github.com/SnishaperTeam/SniShaper)**：Windows / Linux GUI 与 headless CLI，路由理念同源。
- **[Issues / 反馈](https://github.com/SniShaper/lumine-for-android/issues)**：站点访问不稳定或使用问题欢迎在此反馈。

---

## 构建与开发

仓库分为 Android 应用层与 Go 核心模块两层。Go 核心通过 `gomobile` 绑定为 AAR，置于 `android/app/libs/` 下随应用一起编译。

```
android/            Android 应用（Kotlin + Jetpack Compose，Material 3）
  app/src/main/       应用代码、页面与主题
  app/libs/           LumineCore.aar（Go 核心产物，由构建生成）
enimul/             Go 核心：代理与分流（gomobile 绑定入口：./mobile）
tun2socks/          Go 隧道与转发相关模块
scripts/            build-android.ps1、gomobile-bind.ps1
fastlane/           商店元数据（F-Droid / Play）
Makefile            make android → android/app/libs/LumineCore.aar
```

### 构建产物矩阵

| 类型          | 产物                                                                         | 说明                                                        |
| ----------- | -------------------------------------------------------------------------- | --------------------------------------------------------- |
| Debug APK   | `android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk`              | 仅 arm64，日常开发使用                                            |
| Release APK | `android/app/build/outputs/apk/release/app-<abi>-release[(-unsigned)].apk` | 4 个 ABI 分包：`arm64-v8a` / `armeabi-v7a` / `x86` / `x86_64` |
| Go 核心 AAR   | `android/app/libs/LumineCore.aar`                                          | `gomobile bind` 产物，发布流水线中重建                               |

### 构建 Android APK

```bash
git clone https://github.com/SniShaper/lumine-for-android
cd lumine-for-android/android

# Debug 构建
./gradlew assembleDebug

# Release 构建（提供 ANDROID_KEYSTORE_* 环境变量时签名）
./gradlew assembleRelease
```

Windows 下请使用 `gradlew.bat` 而非 `./gradlew`。

### 构建 Go 核心 AAR

需要 Go 工具链、`gomobile`、Android SDK 与 NDK：

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# 在仓库根目录执行
make android
```

`make android` 实际调用 `scripts/gomobile-bind.ps1`（以 `-target=android -androidapi 24` 绑定 `enimul/mobile`，输出 `LumineCore.aar`）。

### 开发环境

- JDK 17+（CI 使用 Temurin 21）
- Android SDK：`compileSdk 36`、`targetSdk 36`、`minSdk 24`（Android 7.0+）
- Android NDK（发布 CI 固定 `30.0.14904198`）与 Build Tools `36.0.0`
- Go 工具链（版本以 `enimul/go.mod` 为准，当前 Go 1.26）+ `golang.org/x/mobile/cmd/gomobile`

---

## 持续集成

- **`android-release.yml`**：在 `main` 分支推送、版本 tag 推送或手动触发时运行。流程会准备 Go + Android SDK + NDK，用 gomobile 重建 `LumineCore.aar`，执行 `assembleRelease`，上传四个 ABI 的 APK 作为工作流产物；tag 推送时创建或更新 GitHub Release，资产重命名为 `lumine-<tag>-app-<abi>-release.apk`。
- **`fastlane.yml`**：对 `fastlane/` 商店元数据做格式校验（push / PR 触碰时触发）。

---

## 平台说明

- 应用支持 **Android 7.0（API 24）及以上**；Release 构建按 ABI 分包以减小单包体积。
- Debug 构建仅产出 `arm64-v8a`；请按真机 / 模拟器架构选择对应分包（或自行重编）。
- 本仓库并非上游 `enimul` 官方仓库，而是面向 Android 的实现与适配版本。部分模式在个别站点可能不稳定，欢迎反馈。

---

## 致谢

本项目受益于以下优秀开源项目的启发：

- [enimul](https://github.com/lzpls/enimul) —— Go 代理 / 分流核心（上游）
- [lumine](https://codeberg.org/PonyCW26/lumine) —— enimul 的前身项目
- [SniShaper](https://github.com/SnishaperTeam/SniShaper) —— 设计理念同源的桌面版项目

## 贡献者

Lumine 作为 SniShaper 项目体系的一部分进行开发（参见 [SniShaperTeam 组织](https://github.com/SniShaperTeam/)）。感谢所有对本仓库做出贡献的开发者：

<div align="center">  
<a href="https://github.com/SniShaper/lumine-for-android/graphs/contributors" target="\_blank">  
<img src="https://contrib.rocks/image?repo=SniShaper/lumine-for-android" alt="Contributors" />  
</a>  
</div>

## 星标历史

<a href="https://www.star-history.com/?repos=SniShaper/lumine-for-android\&type=date">  
 <picture>  
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=SniShaper/lumine-for-android\&type=date\&theme=dark\&legend=top-left" />  
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=SniShaper/lumine-for-android\&type=date\&theme=light\&legend=top-left" />  
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=SniShaper/lumine-for-android\&type=date\&legend=top-left" />  
 </picture>  
</a>

---

## 项目活跃度

![GitHub contributors](https://img.shields.io/github/contributors/SniShaper/lumine-for-android?style=flat\&label=贡献者)

![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SniShaper/lumine-for-android?style=flat\&label=月均提交)

![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/lumine-for-android?style=flat\&label=最近提交)

---

## 开源许可

[GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0）。
