# Lumine for Android

[English](README.md) | [简体中文](README_zh.md)

[![Android](https://img.shields.io/badge/Android-7.0%2B-3DDC84?style=flat&logo=android&logoColor=white)](https://developer.android.com/) [![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![GitHub Release](https://img.shields.io/github/v/release/SniShaper/lumine-for-android?style=flat&logo=github&label=Release)](https://github.com/SniShaper/lumine-for-android/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/lumine-for-android/total?style=flat&logo=github&label=Downloads)](https://github.com/SniShaper/lumine-for-android/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/lumine-for-android?style=flat&logo=git&label=Last%20commit)](https://github.com/SniShaper/lumine-for-android/commits/main) [![CI](https://img.shields.io/github/actions/workflow/status/SniShaper/lumine-for-android/android-release.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SniShaper/lumine-for-android/actions)

**Lumine** is a Clash-style local proxy / VPN client for Android built on the [enimul](https://github.com/lzpls/enimul) Go core (formerly [lumine](https://codeberg.org/PonyCW26/lumine)). It runs as an Android `VPNService` (TUN) tunnel and forwards traffic locally with configurable splitting rules.

The UI is natively built with **Kotlin + Jetpack Compose**, following **Material Design 3** with dynamic color support. The Go core is compiled by `gomobile` into a single AAR, so the app keeps a pure native interface with no embedded WebView. Lumine is the mobile-side companion of the [SniShaper](https://github.com/SnishaperTeam/SniShaper) proxy project.

> Looking for the desktop version? See **[SniShaper](https://github.com/SnishaperTeam/SniShaper)** — a Windows / Linux proxy client with the same enimul-based routing ideas, plus a headless CLI variant.

> Looking for the HarmonyOS version? See **[Lumine for HarmonyOS](https://github.com/SnishaperTeam/lumine-for-harmonyos)** — the HarmonyOS companion built with ArkTS + ArkUI and a portable C++17 core over NAPI (no WebView embedded), featuring VpnExtensionAbility (TUN) tunnels, a local SOCKS5 / HTTP listen mode, subscription management and rule editing.

---

## Features

- **One-tap local proxy (TUN)**: Android `VPNService` tunnel, start / stop from the home screen with a single switch.
- **Subscription management**: import configs from a subscription URL, refresh them, and switch among Clash-style configs on the fly.
- **Rule engine**: dedicated pages to view, create and edit **domain** and **IP / CIDR** rules with multiple proxy modes.
- **Intelligent splitting**: blacklist-driven routing based on GFWList, inherited from the enimul core, plus a flexible Fake-IP implementation.
- **Real-time logs**: level filtering (all / info / error / debug / other), auto-scroll tail, capture toggle and one-tap export.
- **Global settings**: upstream DNS address and core log level.
- **Background keep-alive**: guided setup for accessibility service, auto-start, battery optimization and more on common OEM ROMs.
- **Material Design 3 UI**: dynamic color from the system wallpaper on Android 12+ with a full baseline fallback, light / dark both supported.

---

## Quick Start

### Install from GitHub Releases

Download the [latest release](https://github.com/SniShaper/lumine-for-android/releases) APK that matches your device ABI, then install it (allow "install from unknown sources" when prompted):

| ABI | Typical device |
| --- | --- |
| `arm64-v8a` | Most phones and tablets since ~2017 (recommended) |
| `armeabi-v7a` | Older 32-bit devices |
| `x86_64` | x86_64 emulators such as MuMu / LDPlayer |
| `x86` | Legacy x86 emulators |

### Install from F-Droid

[<img src="https://fdroid.gitlab.io/artwork/badge/get-it-on.png" alt="Get it on F-Droid" height="75">](https://f-droid.org/packages/com.moi.lumine)

Package id: `com.moi.lumine`.

### Configure and Start

1. Open the app and go to **配置订阅 / Subscription**.
2. Add a subscription name and its URL, wait for the import to finish, then tap the config to apply it.
3. Return to the home screen and flip the switch to start the proxy — the tunnel icon appears once `VPNService` is running.
4. Fine-tune behavior on the **Rules** page, and check **Keep-alive** on OEM ROMs if the process gets killed after swiping the app away.

### Install on an Emulator or via adb

```bash
adb install -r app-arm64-v8a-debug.apk   # or any ABI split that matches
adb shell am start -n com.moi.lumine/.MainActivity
```

---

## Documentation

- **[Upstream core (enimul)](https://github.com/lzpls/enimul)** — learn the proxying modes and the config file syntax (kept compatible with upstream).
- **[Desktop counterpart (SniShaper)](https://github.com/SnishaperTeam/SniShaper)** — Windows / Linux GUI and headless CLI sharing similar routing concepts.
- **[Issues / FAQ](https://github.com/SniShaper/lumine-for-android/issues)** — report unstable sites or ask for help.

---

## Build & Development

The repository is split into an Android application and a Go core module. The Go core is bound to Android via `gomobile` and shipped as an AAR under `android/app/libs/`.

```
android/            Android app (Kotlin + Jetpack Compose, Material 3)
  app/src/main/       application code, screens and theme
  app/libs/           LumineCore.aar (Go core, generated)
enimul/             Go core: proxying & routing (gomobile bind entry: ./mobile)
tun2socks/          Go modules for tunnel / forwarding support
scripts/            build-android.ps1, gomobile-bind.ps1
fastlane/           store metadata (F-Droid / Play)
Makefile            make android -> android/app/libs/LumineCore.aar
```

### Build artifact matrix

| Type | Output | Notes |
| --- | --- | --- |
| Debug APK | `android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk` | arm64 only, for daily development |
| Release APKs | `android/app/build/outputs/apk/release/app-<abi>-release[(-unsigned)].apk` | 4 ABI splits: `arm64-v8a` / `armeabi-v7a` / `x86` / `x86_64` |
| Go core AAR | `android/app/libs/LumineCore.aar` | `gomobile bind` output, rebuilt on release CI |

### Build the Android APK

```bash
git clone https://github.com/SniShaper/lumine-for-android
cd lumine-for-android/android

# Debug build
./gradlew assembleDebug

# Release build (signed when ANDROID_KEYSTORE_* env vars are provided)
./gradlew assembleRelease
```

On Windows use `gradlew.bat` instead of `./gradlew`.

### Build the Go core AAR

Requires a Go toolchain, `gomobile`, the Android SDK and an NDK:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# Repository root
make android
```

`make android` wraps `scripts/gomobile-bind.ps1` (bind `enimul/mobile` with `-target=android -androidapi 24` and write `LumineCore.aar`).

### Development environment

- JDK 17+ (CI uses Temurin 21)
- Android SDK: `compileSdk 36`, `targetSdk 36`, `minSdk 24` (Android 7.0+)
- Android NDK (release CI pins `30.0.14904198`) and Build Tools `36.0.0`
- Go toolchain (version pinned by `enimul/go.mod`, currently Go 1.26) + `golang.org/x/mobile/cmd/gomobile`

---

## Continuous Integration

- **`android-release.yml`** — triggered on pushes to `main` / version tags and manual dispatch. It sets up Go + Android SDK + NDK, rebuilds `LumineCore.aar` with gomobile, runs `assembleRelease`, uploads the four ABI APKs as workflow artifacts, and on tag pushes creates / updates a GitHub Release with assets renamed as `lumine-<tag>-app-<abi>-release.apk`.
- **`fastlane.yml`** — validates the `fastlane/` supply metadata on push / PR touching it.

---

## Platform Notes

- The Android app targets **Android 7.0 (API 24)** and above; release builds are split per ABI to keep each APK small.
- Debug builds ship `arm64-v8a` only; install a matching split (or rebuild for your emulator's ABI).
- The repository is not the official upstream `enimul` repository — it is an Android-focused adaptation. Some modes may behave unstably on certain sites; feedback is welcome.

---

## Acknowledgements

This project benefits from the following open-source projects:

- [enimul](https://github.com/lzpls/enimul) — Go proxying / routing core (upstream)
- [lumine](https://codeberg.org/PonyCW26/lumine) — the original project enimul was forked from
- [SniShaper](https://github.com/SnishaperTeam/SniShaper) — desktop counterpart sharing the same design ideas

## Contributors

Lumine is developed as part of the SniShaper project family ([SniShaperTeam](https://github.com/SniShaperTeam/)). Thanks to everyone who contributed to this repository:

<div align="center">
<a href="https://github.com/SniShaper/lumine-for-android/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SniShaper/lumine-for-android" alt="Contributors" />
</a>
</div>

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Flumine-for-android&type=date&legend=top-left">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/lumine-for-android&type=date&theme=dark&legend=top-left" />
<source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/lumine-for-android&type=date&legend=top-left" />
<img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/lumine-for-android&type=date&legend=top-left" />
</picture>
</a>

---

## Project Activity

[![GitHub contributors](https://img.shields.io/github/contributors/SniShaper/lumine-for-android?style=flat&label=Contributors)](https://github.com/SniShaper/lumine-for-android/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SniShaper/lumine-for-android?style=flat&label=Commits%2Fmonth)](https://github.com/SniShaper/lumine-for-android/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/lumine-for-android?style=flat&label=Last%20commit)](https://github.com/SniShaper/lumine-for-android/commits/main)

---

## License

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
