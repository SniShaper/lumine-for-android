# lumine-mobile

`lumine-mobile` 是 [Lumine](https://github.com/moi-si/lumine) 在 Android 平台上的 Clash 风格实现。

它基于 `Lumine` 的核心，结合 Android `VPN/TUN` 方案与移动端界面，提供更贴近 Clash 的使用体验，以及更智能的分流与规则管理能力。
你也可以将其看作[SniShaper](https://github.com/coolapijust/SniShaper)在移动端的功能扩展。

## 特性

- Android 原生实现，kotlin+gomobile，面向移动端
- 基于 `Lumine` 核心的代理与分流方案
- Clash 风格的配置管理界面
- 支持订阅 URL 拉取配置并切换规则
- 独立的规则页面，可查看、编辑和新建规则
- 将工作模式改为基于GFWlist的黑名单驱动，分流更智能
- 灵活的Fake ip实现

## 上游

- Lumine: <https://github.com/moi-si/lumine>

使用了lumine作为核心，并进行了一部分修改；部分模式可能工作不稳定。配置文件语法兼容原版。为优化移动端性能，删除原版规则部分ip段，并修改了分流方式。

## 编译

```bash
git clone https://github.com/coolapijust/lumine-for-android
cd lumine-for-android
```

Android APK 构建：

```bash
cd android
./gradlew assembleDebug
```

Go Mobile Core 构建：

```bash
make android
```

## 说明

本项目不是上游 `Lumine` 官方仓库，而是面向 Android 平台的实现与适配版本。目前处于早期阶段，部分网站可能工作不稳定，欢迎提出反馈。

## 开源许可

GPLv3
