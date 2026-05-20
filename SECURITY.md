# Security Policy

## Supported versions

Only the latest released version receives security fixes.

| Version | Supported |
|---------|-----------|
| latest  | ✅         |
| older   | ❌         |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security reports.**

Use GitHub's private vulnerability reporting to
[open a security advisory](https://github.com/amiwrpremium/macontrol/security/advisories/new).

Please include:

- A description of the issue and the impact
- Steps to reproduce (or a proof-of-concept)
- The macontrol version, macOS version, and chip (M1/M2/M3/M4)

We aim to acknowledge reports within **72 hours** and to ship a patch (or
provide a mitigation) within **30 days** of confirming a vulnerability.

## Scope

In scope:

- The macontrol daemon and CLI
- The LaunchAgent plist and sudoers template shipped with releases
- The Homebrew formula shipped to `amiwrpremium/homebrew-tap`
- The `scripts/install.sh` bootstrap

Out of scope:

- Attacks that require already having the bot token or whitelisted Telegram
  account (that is equivalent to shell access by design)
- Issues in upstream macOS CLIs (`pmset`, `networksetup`, etc.) — please
  report those to Apple
