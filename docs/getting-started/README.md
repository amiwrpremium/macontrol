# Getting started

If you've never run macontrol before, read these in order. Total time
from cold start to bot-responding-in-Telegram is under ten minutes.

## The path

1. **[Installation](installation.md)** — pick a method (Homebrew
   recommended), install the binary. Confirms your Mac is Apple Silicon
   on macOS 11+.
2. **[Telegram credentials](credentials-telegram.md)** — create a bot
   with BotFather, get your Telegram user ID, smoke-test that the token
   works.
3. **[Quickstart](quickstart.md)** — run `macontrol setup`, which
   writes the config, installs the LaunchAgent, and starts the daemon.
4. **[First message](first-message.md)** — send `/start` and walk
   through the home keyboard so you know what each category does.

## Prerequisites

- **Mac** with an Apple Silicon chip (M1, M2, M3, or M4). Intel is not
  supported and the manual install script will refuse to run.
- **macOS 11 Big Sur or newer**. Some features need 12+ (speed test) or
  13+ (Shortcuts runner) — see
  [Reference → Version gates](../reference/version-gates.md).
- **A Telegram account** and the ability to DM
  [@BotFather](https://t.me/BotFather) and
  [@userinfobot](https://t.me/userinfobot).
- **Admin access** on your Mac (for the optional narrow sudoers entry).

## What you'll end up with

- A daemon (`macontrol`) running as a LaunchAgent, auto-starting at
  login and restarting on crash.
- A config at
  `~/Library/Application Support/macontrol/config.env` holding the bot
  token and your user-ID whitelist.
- Rotating logs at `~/Library/Logs/macontrol/macontrol.log`.
- A Telegram bot that responds to `/menu` by showing a one-shot keyboard
  with one button per category (Sound, Display, Wi-Fi, Bluetooth,
  Battery, Power, System, Media, Notify, Tools).

## If anything goes wrong

Each doc in this group has a "verify it worked" section at the bottom.
If a check fails, jump to
[Troubleshooting → Common issues](../troubleshooting/common-issues.md).
