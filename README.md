# lumine-mobile

[简体中文](./README_zh.md)

`lumine-mobile` is a Clash-style Android implementation built on top of [Lumine](https://github.com/moi-si/lumine).

It brings Lumine's core to Android with a `VPN/TUN` pipeline and a mobile-friendly UI, offering a more Clash-like experience with smarter routing and rule management.
You can also view it as a mobile-side functional extension of [SniShaper](https://github.com/coolapijust/SniShaper).

## Features

- Native Android implementation built with Kotlin and `gomobile`
- Proxying and routing powered by Lumine core
- Clash-style configuration management UI
- Subscription URL import and rule switching
- Dedicated rule page for viewing, editing, and creating rules
- Blacklist-driven routing based on GFWlist for smarter traffic splitting
- Flexible Fake IP implementation

## Upstream

- Lumine: <https://github.com/moi-si/lumine>

This project uses Lumine as its core and includes a number of local modifications. Some modes may still be unstable. The configuration file syntax remains compatible with upstream. To optimize mobile performance, some original IP-range rules were removed and the routing behavior was adjusted.

## Build

```bash
git clone https://github.com/coolapijust/lumine-for-android
cd lumine-for-android
```

Build Android APK:

```bash
cd android
./gradlew assembleDebug
```

Build Go mobile core:

```bash
make android
```

## Notes

This repository is not the official upstream Lumine repository. It is an Android-focused implementation and adaptation layer. The project is still in an early stage, and some websites may behave unstably. Feedback is welcome.

## License

GPLv3
