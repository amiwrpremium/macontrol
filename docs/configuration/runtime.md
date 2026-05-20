# Runtime configuration

What macontrol reads at startup, and the small handful of flags
that change daemon behavior.

## Where everything lives

There are exactly two sources of runtime data, and they're
deliberately small:

1. **macOS Keychain** — bot token and user-ID whitelist. Encrypted
   at rest under your account password, with a per-app silent-read
   ACL. Written by `macontrol setup`. See
   [Security → Bot token](../security/bot-token.md).
2. **CLI flags** on `macontrol run` — log level and log file path.
   The LaunchAgent plist installed by Homebrew or
   `macontrol service install` carries any non-default flags.

There is **no `.env` file**, **no config file**, and **no
environment variables** for any of the above. macontrol does not
read `TELEGRAM_BOT_TOKEN`, `ALLOWED_USER_IDS`, `LOG_LEVEL`, or
similar from the process environment. If you set them, they are
ignored.

If a Keychain entry is missing the daemon refuses to start and
prints:

```text
macontrol: missing TELEGRAM_BOT_TOKEN in Keychain.
Run `macontrol setup` to write it
```

## Keychain entries

Two generic-password entries, one per secret:

| Service | Account | Holds |
|---|---|---|
| `com.amiwrpremium.macontrol` | your Unix user | bot token from BotFather |
| `com.amiwrpremium.macontrol.whitelist` | your Unix user | comma-separated Telegram user IDs |

You manage them via macontrol subcommands, not by editing files:

| Task | Command |
|---|---|
| Initial setup (both entries) | `macontrol setup` |
| Inspect token presence | `macontrol doctor` |
| Replace token | `macontrol token set` |
| Remove token | `macontrol token clear` |
| Re-grant Keychain ACL after binary moved | `macontrol token reauth` |
| List whitelist | `macontrol whitelist list` |
| Add a user | `macontrol whitelist add 123456789` |
| Remove a user | `macontrol whitelist remove 123456789` |
| Empty the whitelist | `macontrol whitelist clear` |

See [Whitelist](whitelist.md) for the day-to-day flow.

## CLI flags on `macontrol run`

The daemon entry point — `macontrol run` — accepts these flags.
The Homebrew formula's service block and the bundled LaunchAgent
plist set `--log-level=info` explicitly; everything else uses
defaults.

### `--log-level`

Log verbosity for `slog` output.

| Property | Value |
|---|---|
| Type | string (one of `debug`, `info`, `warn`, `error`) |
| Default | `info` |
| Example | `--log-level=debug` |

`debug` adds:

- Per-update routing decisions
- Callback parses
- Flow state transitions
- Subprocess command lines and exit codes

Production should stay on `info`. Use `debug` while diagnosing a
specific issue, then switch back.

Unknown values silently fall back to `info`.

### `--log-file`

Path the daemon writes logs to. Lumberjack rotates the file at
10 MB, keeping 5 backups for 30 days.

| Property | Value |
|---|---|
| Type | string (filesystem path) |
| Default | `~/Library/Logs/macontrol/macontrol.log` |
| Example | `--log-file=/Users/me/Desktop/macontrol-debug.log` |

Pass an empty string (`--log-file=""`) to skip the file and write
to stderr. Useful when running the daemon in the foreground for
ad-hoc debugging.

If the parent directory doesn't exist or isn't writable, lumberjack
fails on the first log write and the daemon exits.

## Changing flags on a running daemon

Flags are read at startup. To change them:

```bash
brew services stop macontrol
# edit ~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist
brew services start macontrol
```

Or for an ad-hoc one-shot run:

```bash
macontrol service stop
macontrol run --log-level=debug
# (in another terminal, watch the log)
# Ctrl-C when done
macontrol service start
```

## What's not configurable

Some constants are deliberately not surfaced as flags. If you need
to change them, patch the code:

| Setting | Hardcoded as | Where |
|---|---|---|
| Subprocess timeout | 15 seconds | `internal/runner/runner.go` (`DefaultTimeout`) |
| Flow inactivity timeout | 5 minutes | `cmd/macontrol/daemon.go` (`flows.NewRegistry`) |
| Shortmap TTL | 15 minutes | `cmd/macontrol/daemon.go` (`callbacks.NewShortMap`) |
| Log rotation: max size | 10 MB | `cmd/macontrol/daemon.go` (`lumberjack.Logger.MaxSize`) |
| Log rotation: keep | 5 backups, 30 days | `lumberjack.Logger.MaxBackups` / `MaxAge` |

These could be made configurable via flags in a future release if
there's a real use case — open an issue.
