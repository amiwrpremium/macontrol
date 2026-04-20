# Logs

macontrol writes structured logs via Go's `slog` and rotates them with
`lumberjack`. Both human-readable and grep-friendly.

## Where

```text
~/Library/Logs/macontrol/macontrol.log               # current
~/Library/Logs/macontrol/macontrol.log.1.gz          # rotated (most recent)
~/Library/Logs/macontrol/macontrol.log.2.gz
…
~/Library/Logs/macontrol/macontrol.log.5.gz          # oldest kept
```

Plus, if running under launchd:

```text
~/Library/Logs/macontrol/macontrol.err.log           # process-level stderr (panics, runtime errors)
```

The `.err.log` is a separate stream because launchd captures `stderr`
into its own file (configured in the plist's `StandardErrorPath`).
Most error output ends up in the structured `.log` file via slog;
panics that escape recovery middleware land in `.err.log`.

## Format

Default format is `slog.NewTextHandler` — a key-value text format that
both humans and `grep`/`awk` parse cleanly:

```text
time=2026-04-20T10:22:35.123-04:00 level=INFO msg="macontrol starting" version=v0.1.0 commit=abc1234 config=/Users/you/Library/Application Support/macontrol/config.env log=/Users/you/Library/Logs/macontrol/macontrol.log
time=2026-04-20T10:22:35.301-04:00 level=INFO msg=capabilities summary="macOS 15.3 · 3/3 version-gated features available"
time=2026-04-20T10:22:35.302-04:00 level=INFO msg="bot started"
time=2026-04-20T10:22:36.701-04:00 level=DEBUG msg=command text=/menu from=123456789
time=2026-04-20T10:22:38.892-04:00 level=DEBUG msg=callback data=snd:open from=123456789
```

## Rotation

Handled by `gopkg.in/natefinch/lumberjack.v2`:

| Property | Value |
|---|---|
| Max size | 10 MB |
| Max backups | 5 |
| Max age | 30 days |
| Compress backups | yes (gzip) |

When the current `macontrol.log` exceeds 10 MB, lumberjack:

1. Renames it to `macontrol.log.1`.
2. Gzips `macontrol.log.1` → `macontrol.log.1.gz`.
3. Shifts older `.gz` files up (`.1.gz` → `.2.gz`, etc.).
4. Drops anything older than 30 days OR beyond the 5th backup.
5. Opens a fresh `macontrol.log`.

In practice, a quietly-running daemon emits maybe 1 MB / week of logs,
so rotation is rare.

## Log levels

Configured via `LOG_LEVEL` env var. See [Configuration → env.md](../configuration/env.md#log_level).

- `debug` — every routing decision, every callback parse, every flow
  state transition, every subprocess command line and exit code.
- `info` (default) — startup, lifecycle events, rejected updates,
  significant actions.
- `warn` — recoverable problems (handler errors, missing optional
  deps).
- `error` — unrecoverable problems before exit.

For diagnosing a specific issue, switch to `debug` temporarily:

```bash
LOG_LEVEL=debug macontrol run        # foreground, immediate
# or persistent:
echo 'LOG_LEVEL=debug' >> ~/Library/Application\ Support/macontrol/config.env
brew services restart macontrol
```

Switch back after, since `debug` is verbose:

```dotenv
LOG_LEVEL=info
```

## Tailing live

```bash
tail -f ~/Library/Logs/macontrol/macontrol.log
```

Or via the built-in helper:

```bash
macontrol service logs
```

Both call `tail -n 200 -f` on the current file. If lumberjack rotates
during the tail, you'll see "file truncated" and may need to re-open.

## Common log entries

A reference for what you'll see during normal operation.

### Startup

```text
INFO  msg="macontrol starting"  version=v0.1.0 commit=abc1234 config=… log=…
INFO  msg=capabilities  summary="macOS 15.3 · 3/3 version-gated features available"
INFO  msg="bot started"
```

### Routing a command

```text
DEBUG msg=command  text=/menu  from=123456789
```

### Routing a callback

```text
DEBUG msg=callback  data=snd:up:5  from=123456789
```

### Rejected non-whitelisted user

```text
WARN  msg="rejected update from non-whitelisted user"  sender=999999999
```

### Telegram transport error (recoverable)

```text
ERROR msg="telegram transport error"  err="context deadline exceeded"
```

These usually mean the long-poll request timed out and the library is
re-issuing it. Harmless unless they happen continuously.

### Subprocess failure

```text
WARN  msg="callback dispatch"  err="brightness -l: exit status 127: brightness: command not found"
```

Tells you the dashboard couldn't read brightness because the `brightness`
brew formula isn't installed.

### Panic recovered

```text
ERROR msg="panic in handler"  panic="runtime error: index out of range [3] with length 3"
```

The recover middleware caught a panic. The user's request was dropped
(no reply), but the daemon stays alive. File a bug report with the
log line if you see one of these.

## Console.app

macOS's Console.app reads `~/Library/Logs/` automatically. Open Console,
expand your username, expand `macontrol`, and you'll see the log file
listed alongside Apple's app logs. You can use Console's filtering UI
to grep by level, by message text, etc.

This is convenient if you don't want to leave Telegram open or live in
a terminal.

## Adding more detail to a specific feature

If you're investigating a specific category and want extra logging
without enabling debug everywhere, the cleanest path is to add a
temporary `slog.Debug` line in the relevant handler and recompile.
The slog framework is structured, so:

```go
d.Logger.Debug("handling sound action", "action", data.Action, "args", data.Args)
```

Adds context-rich entries you can grep for:

```bash
grep "handling sound action" ~/Library/Logs/macontrol/macontrol.log
```

## Forwarding logs elsewhere

macontrol writes to a local file. If you want to ship logs to syslog,
Datadog, Loki, etc., the simplest path is a sidecar tail:

```bash
tail -F ~/Library/Logs/macontrol/macontrol.log | loki-pusher --label=app=macontrol
```

(Replace `loki-pusher` with whatever tool your destination uses.)

There's no built-in syslog/journald support — macOS doesn't have
journald and Mac syslog routing is fiddly enough that we've left it
out by default.

## Privacy

Logs include:

- Telegram **user IDs** of senders (numeric, not usernames).
- Slash command text and callback data (not message bodies for free-text
  flows — those aren't logged).
- Subprocess command lines (which include arguments like Wi-Fi SSIDs).

Do not paste raw logs into a public bug report without redacting your
user IDs. Replace them with `123456789` placeholders.
