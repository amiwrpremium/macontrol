# macontrol documentation

A Telegram bot that controls your Mac — volume, brightness, network,
power, media, notifications, and more. One Go binary, Apple Silicon
only, zero runtime dependencies on the Mac.

This directory is the full reference. The top-level
[`README.md`](../README.md) is the 30-second overview.

## How to use these docs

- **First time here?** Start with
  [Getting started → Installation](getting-started/installation.md),
  then [Quickstart](getting-started/quickstart.md).
- **Configuring the bot?** See [Configuration](#configuration).
- **Something broke?** Start with
  [Troubleshooting → Common issues](troubleshooting/common-issues.md).
- **Writing code?** See [Development](#development).

## Table of contents

### Getting started

- [Installation](getting-started/installation.md) — Homebrew, manual script, building from source
- [Telegram credentials](getting-started/credentials-telegram.md) — bot token + your user ID + smoke-test
- [Quickstart](getting-started/quickstart.md) — five-minute first run end to end
- [First message](getting-started/first-message.md) — walking through the home keyboard after `/start`

### Usage

- [UX model](usage/ux-model.md) — one-shot reply keyboard, inline edits, flows, confirms
- [Slash commands](usage/commands.md) — `/start`, `/menu`, `/status`, `/help`, `/cancel`, `/lock`, `/screenshot`
- [Categories](usage/categories/README.md) — every button in every dashboard

### Configuration

- [Environment variables](configuration/env.md) — `TELEGRAM_BOT_TOKEN`, `ALLOWED_USER_IDS`, `LOG_LEVEL`
- [File locations](configuration/file-locations.md) — where config, logs, plist, and cache live
- [Whitelist](configuration/whitelist.md) — adding and removing allowed users

### Permissions

- [TCC grants](permissions/tcc.md) — Screen Recording, Accessibility, Camera, Automation
- [Sudoers](permissions/sudoers.md) — narrow `/etc/sudoers.d/macontrol` entry

### Operations

- [Running](operations/running.md) — foreground, `brew services`, `launchctl`
- [Logs](operations/logs.md) — paths, rotation, log levels
- [Doctor](operations/doctor.md) — `macontrol doctor` output explained
- [Upgrades](operations/upgrades.md) — `brew upgrade` and manual

### Architecture

- [Overview](architecture/overview.md) — dispatch loop and data flow
- [Project layout](architecture/project-layout.md) — file-by-file walk
- [Design decisions](architecture/design-decisions.md) — the non-obvious whys
- [Testing](architecture/testing.md) — coverage, fakes, the test harness

### Reference

- [CLI](reference/cli.md) — every subcommand and flag
- [Callback protocol](reference/callback-protocol.md) — `<ns>:<action>:<arg>` + shortmap
- [macOS CLI mapping](reference/macos-cli-mapping.md) — feature → backing command table
- [Version gates](reference/version-gates.md) — which features need which macOS

### Security

- [Bot token](security/bot-token.md) — secret hygiene and rotation
- [Threat model](security/threat-model.md) — what attackers can do at each access level
- [Reporting vulnerabilities](security/reporting-vulnerabilities.md)

### Troubleshooting

- [Common issues](troubleshooting/common-issues.md) — symptom → cause → fix
- [Permission issues](troubleshooting/permission-issues.md) — TCC and sudoers errors
- [Telegram errors](troubleshooting/telegram-errors.md) — Bot API HTTP codes

### Development

- [Contributing](development/contributing.md) — workflow and PR process
- [Conventional Commits](development/conventional-commits.md) — types, scopes, examples
- [Adding a capability](development/adding-a-capability.md) — the 6-file recipe
- [Testing](development/testing.md) — running tests and coverage
- [CI](development/ci.md) — workflows and what each job does
- [Releasing](development/releasing.md) — release-please + GoReleaser + tap
- [Roadmap](development/roadmap.md) — planned work and non-goals

### [FAQ](faq.md)
