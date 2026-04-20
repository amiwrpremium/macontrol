# macontrol

> Control your Mac from Telegram — system, media, network, power, and more.
> Apple Silicon · Go · single binary.

[![CI](https://github.com/amiwrpremium/macontrol/actions/workflows/ci.yml/badge.svg)](https://github.com/amiwrpremium/macontrol/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/amiwrpremium/macontrol?sort=semver)](https://github.com/amiwrpremium/macontrol/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/amiwrpremium/macontrol.svg)](https://pkg.go.dev/github.com/amiwrpremium/macontrol)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://www.conventionalcommits.org)

`macontrol` is a tiny Go daemon that runs on your Mac and exposes a **menu-first Telegram bot** for remote control: change volume/brightness, toggle Wi-Fi/Bluetooth, read battery & system stats, take screenshots, send desktop notifications, lock/sleep/restart, and more.

## Features

| Category | What you can do |
|---|---|
| 🔊 Sound | Volume ± / set / mute / max |
| 💡 Display | Brightness ± / set, trigger screen saver |
| 🔋 Battery | Percent, charging state, health, cycle count |
| 📶 Wi-Fi | Toggle, info, join network, DNS presets, speed test |
| 🔵 Bluetooth | Toggle, list, connect/disconnect paired devices |
| ⚡ Power | Lock, sleep, restart, shutdown, logout, keep-awake |
| 🖥 System | macOS/HW info, thermal pressure, memory, CPU, top N, kill proc |
| 📸 Media | Full/display/window screenshot, screen recording, webcam photo |
| 🔔 Notify | Desktop notification (terminal-notifier → osascript fallback), text-to-speech |
| 🛠 Tools | Clipboard get/set, timezone pick, time sync, disks list, run any Shortcut |

See the [capability catalog](docs/capabilities.md) for exact commands and version gates.

## Install

### Homebrew (recommended)

```bash
brew install amiwrpremium/tap/macontrol
macontrol setup                 # interactive wizard
brew services start macontrol
```

### Manual

```bash
curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | sh
macontrol setup
macontrol service install       # writes LaunchAgent plist, launchctl-loads it
```

Apple Silicon, macOS 11 (Big Sur) or newer. Intel is not supported.

## Quick start

1. Create a bot with [@BotFather](https://t.me/BotFather), copy the token.
2. Get your Telegram user ID from [@userinfobot](https://t.me/userinfobot).
3. `macontrol setup` — paste both, the wizard writes config, installs the LaunchAgent, and offers a narrow `/etc/sudoers.d/macontrol` entry for the few commands that need sudo.
4. Send `/start` to your bot. Tap away.

See [docs/permissions.md](docs/permissions.md) for the one-time TCC grants (Screen Recording, Accessibility, Camera) that unlock screenshots, app listing, and webcam photos.

## UX model

- `/menu` or `/start` shows a **home reply keyboard** with one button per category.
- Each category opens an **inline keyboard** with buttons like `-5 / -1 / MUTE / +1 / +5 / MAX`. Button presses edit the same message in place, so the dashboard is live.
- Destructive actions (shutdown, restart, logout) require a **confirm sub-keyboard**.
- Actions needing free-text input (set exact volume, join wifi, pick timezone) drop into a **flow** — a 5-minute conversation the bot drives. `/cancel` aborts.

## Configuration

macontrol reads `~/Library/Application Support/macontrol/config.env`:

```dotenv
TELEGRAM_BOT_TOKEN=123:abc...
ALLOWED_USER_IDS=123456789,987654321
LOG_LEVEL=info
```

Logs rotate in `~/Library/Logs/macontrol/macontrol.log`.

## Development

```bash
make lint test                  # golangci-lint + go test -race
make build                      # cross-compile for darwin/arm64
make run                        # run locally against a dev bot token
```

Conventional Commits are required for PR titles — see [CONTRIBUTING.md](CONTRIBUTING.md). Scopes mirror the project tree (`feat(sound):`, `fix(wifi):`…).

Releases are cut by [`release-please`](https://github.com/googleapis/release-please) — merge the version PR, GoReleaser builds the tarball and updates the Homebrew tap automatically.

## Security

Never share your bot token. macontrol enforces a hard user-ID whitelist; non-whitelisted updates are dropped silently. Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
