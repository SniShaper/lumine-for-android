# Security Policy

## Supported Versions

Security fixes are released for the latest stable release and land on `main`.
Older versions are not patched retroactively.

| Version | Supported |
| --- | --- |
| Latest stable (0.9.x) | Yes |
| Older releases | No |

## Reporting a Vulnerability

Please do **not** open a public issue for security problems.

1. Preferred: use GitHub private vulnerability reporting (Security -> Report a
   vulnerability) on this repository.
2. Fallback: email `youmao2023@outlook.com` with the details below. If the
   message is sensitive, encrypt it to the signing key used for this project:

```
Key ID:   6DB18B98EA8946FB109BA6DF09B0556A83AC9B7D
```

Please include:

- the affected component and version (app release, commit hash, or F-Droid
  build);
- the Android version and device where it reproduces;
- a minimal description of the vulnerability and, if possible, a proof of
  concept;
- whether the issue has been discussed publicly.

You can expect an acknowledgment within 3 business days. We will confirm the
vulnerability, assess its impact, prepare a fix, and publish details only after
the fixed release is available so users can update first.

## Scope

This policy covers the Android app (`android/`), the engine bind surface
(`enimul/mobile`), the engine core (`enimul/internal/core`), and the vendored
`tun2socks` code used by the app. Issues in upstream projects should be
reported to their own maintainers. Network reachability or blocking behavior
of specific destinations is not a security issue.
