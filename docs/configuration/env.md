# Environment variables

Every variable macontrol reads, with type, default, requirement, and
example.

## Required

### `TELEGRAM_BOT_TOKEN`

The bot token from BotFather.

| Property | Value |
|---|---|
| Type | string |
| Default | none |
| Required | yes |
| Example | `123456789:AAE-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456` |

Format is `<bot-id>:<secret>`. The bot ID is the numeric prefix, the
secret is everything after the colon. Treat the entire string as a
password — anyone with it can impersonate your bot.

If missing or empty, the daemon exits with:

```text
macontrol: missing required config: TELEGRAM_BOT_TOKEN.
Run `macontrol setup` to write them, or edit the config file directly.
```

See [Getting started → Telegram credentials](../getting-started/credentials-telegram.md)
for how to create one.

### `ALLOWED_USER_IDS`

Comma-separated list of numeric Telegram user IDs allowed to control
the bot.

| Property | Value |
|---|---|
| Type | comma-separated list of `int64` |
| Default | none |
| Required | yes |
| Example | `123456789,987654321` |

Whitespace around the commas is fine: `123, 456, 789` parses the same
as `123,456,789`. Trailing commas are tolerated.

The daemon enforces this strictly: any incoming Telegram update from a
user not on this list is dropped silently, with a single line written
to the log:

```text
WARN  rejected update from non-whitelisted user  sender=999999999
```

There's no "owner vs. operator" role distinction — every user on the
list has the same access.

If empty, the daemon exits with the same friendly error as missing
token.

See [Whitelist](whitelist.md) for how to add or remove users without
re-running setup.

## Optional

### `LOG_LEVEL`

Log verbosity for `slog` output.

| Property | Value |
|---|---|
| Type | string (one of `debug`, `info`, `warn`, `error`) |
| Default | `info` |
| Required | no |
| Example | `LOG_LEVEL=debug` |

`debug` adds:

- Per-update routing decisions (which router got the dispatch)
- Callback parses
- Flow state transitions
- Subprocess command lines and exit codes

Production should stay on `info`. Use `debug` while diagnosing a
specific issue, then switch back.

Unknown values silently fall back to `info`.

### `MACONTROL_CONFIG`

Override path for the config file.

| Property | Value |
|---|---|
| Type | string (filesystem path) |
| Default | `~/Library/Application Support/macontrol/config.env` |
| Required | no |
| Example | `MACONTROL_CONFIG=/etc/macontrol/test.env` |

Useful for testing or running multiple daemons.

### `MACONTROL_LOG`

Override path for the log file.

| Property | Value |
|---|---|
| Type | string (filesystem path) |
| Default | `~/Library/Logs/macontrol/macontrol.log` |
| Required | no |
| Example | `MACONTROL_LOG=/var/log/macontrol.log` |

If empty or unset, the default is used. If set to a path the daemon
can't write to, startup fails before bot init.

## Worked example

A minimal `config.env`:

```dotenv
TELEGRAM_BOT_TOKEN=123456789:AAE-real-token-here
ALLOWED_USER_IDS=123456789
LOG_LEVEL=info
```

A multi-user dev setup with extra logging:

```dotenv
TELEGRAM_BOT_TOKEN=123456789:AAE-real-token-here
ALLOWED_USER_IDS=123456789,987654321,555444333
LOG_LEVEL=debug
MACONTROL_LOG=/Users/me/Desktop/macontrol-debug.log
```

## Validation rules

All applied at startup. If any fail, the daemon exits with code 1 and
a diagnostic message:

| Variable | Rule |
|---|---|
| `TELEGRAM_BOT_TOKEN` | non-empty |
| `ALLOWED_USER_IDS` | non-empty; each comma-separated value parses as int64 |
| `LOG_LEVEL` | optional; if set, one of `debug`, `info`, `warn`, `error` (silent fallback to `info` for unknown) |
| `MACONTROL_CONFIG` | optional; if set, file must exist |
| `MACONTROL_LOG` | optional; if set, parent directory must be writable |

## What's not configurable

Some things are deliberately not env-configurable. If you need to change
them, patch the code:

| Setting | Hardcoded as | Where |
|---|---|---|
| Subprocess timeout | 15 seconds | `internal/runner/runner.go` (`DefaultTimeout`) |
| Flow inactivity timeout | 5 minutes | `cmd/macontrol/daemon.go` (`flows.NewRegistry`) |
| Shortmap TTL | 15 minutes | `cmd/macontrol/daemon.go` (`callbacks.NewShortMap`) |
| Log rotation: max size | 10 MB | `cmd/macontrol/daemon.go` (`lumberjack.Logger.MaxSize`) |
| Log rotation: keep | 5 backups, 30 days | `lumberjack.Logger.MaxBackups` / `MaxAge` |

These could be made configurable via env vars in a future release if
there's a real use case — open an issue.
