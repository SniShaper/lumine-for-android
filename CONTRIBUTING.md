# Contributing Guide

Thank you for your interest in **Lumine for Android**! We welcome all forms of
contribution, including code, rules, documentation, testing, and feedback.

This guide will help you get started quickly.

## How to Contribute

You can contribute in the following ways:

- Report bugs
- Suggest new features
- Improve documentation
- Submit code (bug fixes, optimizations, new features)
- Improve the default rule/configuration sets
- Help with testing and feedback

### Submitting Issues

Please use the existing issue templates whenever possible:

- **Bug Report**: describe the problem, app version, Android version and
  device, reproduction steps, and attach logs where possible
- **Feature Request**: explain the requirement, use cases, and expected
  behavior

Before opening a new issue, please search existing issues to avoid duplicates.

### Submitting Pull Requests

1. Fork this repository
2. Create a new branch (use clear names such as `fix/xxx` or `feat/xxx`)
3. Make your changes and verify the project builds (see below)
4. Write clear, conventional commit messages (English)
5. Push to your fork
6. Open a Pull Request against the `main` branch of the upstream repository

In the PR description, please include:

- What was changed
- Why the change is needed
- Related issue (use `Fixes #number` or `Closes #number` only when the PR
  actually resolves it)
- Verification notes (device/Android version tested, engine tests run, etc.)

## Development Environment

This project has three parts that build independently:

| Part | Stack | How to build/verify |
| --- | --- | --- |
| Android app | Kotlin + Jetpack Compose (Material 3), minSdk 24, target/compile SDK 36 | `cd android && ./gradlew :app:compileDebugKotlin` |
| Engine core | Go (see `enimul/go.mod`) | `cd enimul && go build ./... && go test ./internal/core/...` |
| tun2socks fork | Go (see `tun2socks/go.mod`) | `cd tun2socks && go build ./...` |

Prerequisites: JDK 17+, Android SDK with platform 36, and Go matching the
module `go` directives. If you change Go code under `enimul/`, the
`LumineCore.aar` used by the Android app must be regenerated:
`make android` (requires `gomobile` and the Android NDK; see
`scripts/gomobile-bind.ps1`).

Gradle dependency mirrors are configured in `android/build.gradle` (Tencent
mirror first, CERNET as fallback).

## Project Structure

- `android/` — Android app: `LumineVpnService` (VPN service and lifecycle),
  Compose UI under `ui/screens`, `repository/` (persistence), `keepalive/`
  (background keep-alive components)
- `enimul/` — engine (vendored from [lzpls/enimul](https://github.com/lzpls/enimul)):
  `internal/core` for policy matching / DNS / TLS fragmentation / routing,
  `mobile` is the Go Mobile bind surface exposed to Kotlin
- `tun2socks/` — vendored tun2socks engine used at runtime
- `scripts/gomobile-bind.ps1` — AAR build script
- `fastlane/` — F-Droid / release metadata

## Rules

- Commit messages and GitHub comments are written in **English** and use
  conventional prefixes (`feat:`, `fix:`, `deps:`, `build:`, `docs:`, `ci:`,
  `perf:`, `refactor:`, `test:`).
- Commits are **GPG-signed** (`git commit -S`).
- Keep one logical change per commit and stay within the requested scope.
- Do not add code comments unless necessary; match the surrounding style.
- Keep behavior changes backed by verification notes; engine routing changes
  depend on real network conditions and should be tested on device.

## License

By contributing, you agree that your contributions are licensed under the
[AGPL-3.0](LICENSE) license of this repository.
