# Configuration

macontrol's runtime config is small — three environment variables plus
a few macOS-idiomatic file paths.

## What's here

- **[Environment variables](env.md)** — `TELEGRAM_BOT_TOKEN`,
  `ALLOWED_USER_IDS`, `LOG_LEVEL`, plus optional overrides.
- **[File locations](file-locations.md)** — where the config file,
  logs, LaunchAgent plist, and cache live on macOS.
- **[Whitelist](whitelist.md)** — adding or removing allowed Telegram
  user IDs.

## At a glance

```dotenv
# ~/Library/Application Support/macontrol/config.env
TELEGRAM_BOT_TOKEN=123456789:AAE-aBcDeFgHiJk...
ALLOWED_USER_IDS=123456789,987654321
LOG_LEVEL=info
```

That's the entire config surface for normal use. Everything else has
defaults that work.

## How config is loaded

1. The daemon checks `MACONTROL_CONFIG` env var first. If set, that
   path is the config file. Required variables come from this file.
2. Otherwise, the default path
   `~/Library/Application Support/macontrol/config.env` is read if
   present.
3. After file load, the daemon reads from process env (so anything
   exported in the shell at start time also applies, overriding the
   file).
4. `caarlos0/env` validates the resulting env: required fields must
   be set and non-empty, comma-separated lists are split.
5. If validation fails, the daemon exits with a friendly error
   pointing at `macontrol setup`.

You won't normally use `MACONTROL_CONFIG` — it's a hook for testing or
running multiple daemons against different bots from the same Mac.

## How to change config

Three ways, in order of preference:

1. **`macontrol setup --reconfigure`** — re-runs the wizard, overwrites
   the config file. Easiest.
2. **Edit the config file directly** with any text editor.
   `~/Library/Application Support/macontrol/config.env` is plain
   `KEY=value` lines (dotenv syntax).
3. **Export variables in the shell** before starting the daemon.
   Useful for one-off testing without touching the file.

After changing config, restart the daemon for it to take effect:

```bash
brew services restart macontrol
# or, if installed manually:
macontrol service stop && macontrol service start
```

The daemon does not hot-reload config.
