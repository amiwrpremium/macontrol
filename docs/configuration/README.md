# Configuration

macontrol's runtime config is small: two secrets in the macOS
Keychain, two optional CLI flags on the daemon.

## What's here

- **[Runtime configuration](runtime.md)** — Keychain entries and
  the `--log-level` / `--log-file` flags on `macontrol run`.
- **[File locations](file-locations.md)** — where logs, the
  LaunchAgent plist, and sudoers entry live on macOS.
- **[Whitelist](whitelist.md)** — adding or removing allowed
  Telegram user IDs.

## At a glance

Everything lives in the Keychain, written by the setup wizard:

```bash
macontrol setup           # writes both secrets to the Keychain
brew services start macontrol
```

No `.env` file. No config file. No environment variables.

## How config is loaded

1. On startup, `macontrol run` reads two Keychain entries:
   `com.amiwrpremium.macontrol` (token) and
   `com.amiwrpremium.macontrol.whitelist` (user IDs).
2. If either is missing, the daemon exits with a friendly error
   pointing at `macontrol setup`.
3. Non-secret runtime knobs (`--log-level`, `--log-file`) come
   from flags on the `run` subcommand — not from env vars. The
   LaunchAgent plist and Homebrew formula's service block carry
   any non-default values.

## How to change config

| What | Command |
|---|---|
| Redo the token + whitelist wizard | `macontrol setup --reconfigure` |
| Replace just the token | `macontrol token set` |
| Add/remove a whitelisted user | `macontrol whitelist add 123`, `macontrol whitelist remove 123` |
| Change log level | edit `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist`, restart |

After any of these, restart the daemon for it to take effect:

```bash
brew services restart macontrol
# or, if installed manually:
macontrol service stop && macontrol service start
```

The daemon does not hot-reload configuration.
