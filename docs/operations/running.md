# Running

Three ways to run macontrol, in order from quickest to most robust.

## 1. Foreground (development / debugging)

```bash
macontrol run
# or just
macontrol
```

Runs the daemon attached to your terminal. Logs go to **stderr** in
this mode (also still rotated to `~/Library/Logs/macontrol/`).

Use when:

- Iterating on changes locally
- Watching real-time logs without `tail -f`
- Diagnosing a startup failure (errors print before exit)

`Ctrl-C` (SIGINT) stops it cleanly. The bot sends a "shutting down"
ping to whitelisted users (if implemented for graceful exit) and
flushes the log.

## 2. `brew services` (recommended for installed users)

```bash
brew services start macontrol
brew services restart macontrol
brew services stop macontrol
brew services info macontrol
```

`brew services` wraps `launchctl` with a sane interface. It writes a
plist to `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist`
that's the same shape as the one `macontrol service install` would
write — just managed by Homebrew.

Use when:

- You installed via Homebrew (`brew install amiwrpremium/tap/macontrol`)
- You want auto-start at login

`brew services info macontrol` shows:

```text
Name:          macontrol
User:          you
PID:           7891
Status:        running
File:          /Users/you/Library/LaunchAgents/com.amiwrpremium.macontrol.plist
```

Status values: `started` (running), `stopped` (loaded but not running),
`error` (last exit was non-zero), `none` (not loaded).

## 3. `macontrol service` (manual install)

```bash
macontrol service install      # write plist + bootstrap
macontrol service start        # bootstrap (if uninstall'd previously)
macontrol service stop         # bootout
macontrol service status       # launchctl print
macontrol service logs         # tail -f the log file
macontrol service uninstall    # bootout + rm plist
```

Use when:

- You installed manually (curl install.sh) and don't have brew services
  available
- You want to script around it (each command is a single launchctl call
  with a clean exit code)

`install` writes the plist with the **current binary path** (resolved
via `os.Executable()` from inside the running `macontrol` process).
That means: if you move the binary, re-run `service install` to update
the plist.

## What gets written

The plist that ends up in `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.amiwrpremium.macontrol</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/macontrol</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>/Users/you/Library/Logs/macontrol/macontrol.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/you/Library/Logs/macontrol/macontrol.err.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
```

Key fields:

- **Label** — must be globally unique on your Mac. macontrol uses
  reverse-DNS form `com.amiwrpremium.macontrol`.
- **ProgramArguments** — argv. First element is the binary path, second
  is the `run` subcommand.
- **RunAtLoad** — start the daemon as soon as the plist is loaded
  (i.e. at login).
- **KeepAlive** — restart on exit, regardless of exit code.
- **ProcessType: Interactive** — tells macOS this is a user-facing
  process and shouldn't be aggressively throttled by App Nap.
- **ThrottleInterval: 10** — wait at least 10 s between restart
  attempts. Prevents fast crash-restart loops from pegging your CPU.
- **StandardOutPath / StandardErrorPath** — files to redirect stdout
  and stderr. macontrol uses slog, which writes to its own log file
  (lumberjack-rotated), but Go's runtime panics still go through the
  process's stderr — capturing them here.
- **EnvironmentVariables → PATH** — explicitly set so subprocess
  invocations of `brightness`, `blueutil`, etc. find brew binaries.
  Without this, launchd-spawned processes get a minimal PATH that
  often doesn't include `/opt/homebrew/bin`.

## Restart cycle behavior

When the daemon exits (clean or crash) and KeepAlive is on:

1. launchd notices the exit.
2. Waits `ThrottleInterval` (10 s) since the **previous start**.
3. Spawns a new process.

For consecutive crashes within 10 s, you'll see roughly one restart
every 10 s. For an "exit after handling one update" bug (impossible
with current code, hypothetically), launchd would let it stabilize
once it's been alive >10 s.

If macontrol exits **cleanly** (e.g. SIGTERM from `launchctl bootout`),
launchd does NOT restart it — that's `bootout`'s point.

## Multiple daemons

Don't run two macontrol daemons against the same bot token. Telegram's
long-polling delivers each update to whichever client is polling at
the moment, so updates are randomly split between the two. The bot
will appear flaky.

If you really want two daemons (for testing, or for a dev + prod bot),
each must have its own bot token (and its own LaunchAgent label). The
code path is `MACONTROL_CONFIG=path/to/test.env macontrol run` for the
dev side, started manually or via a custom plist.

## Wake from sleep

When the Mac sleeps and wakes:

- The daemon process **stays alive** (sleep is a global pause, not a
  per-process kill).
- Telegram's long-poll request is suspended along with the daemon.
- On wake, the long-poll resumes and either picks up where it left
  off or times out (Telegram closes long-polls after ~50 s) and
  re-issues.

Net effect: about a 5–60 s delay after wake before the bot responds
to the first message. Subsequent messages are instant.

## Network changes

If your IP changes (Wi-Fi roam, VPN connect/disconnect), the existing
long-poll connection breaks and the underlying HTTP client retries.
You may see a 1–10 s gap in responsiveness after a network change.
Nothing in macontrol's code path explicitly handles this — Go's net
stack and the `go-telegram/bot` library do it transparently.

## Manual launchctl (for the curious)

If you want to bypass `brew services` and `macontrol service` and use
launchctl directly:

```bash
# Load (start)
launchctl bootstrap gui/$UID ~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist

# Unload (stop)
launchctl bootout gui/$UID/com.amiwrpremium.macontrol

# Status
launchctl print gui/$UID/com.amiwrpremium.macontrol

# Force-restart without unload/reload
launchctl kickstart -k gui/$UID/com.amiwrpremium.macontrol
```

The `gui/$UID` domain is the per-user GUI session domain. Use this
form, not the older `launchctl load -w` syntax (deprecated).

## Stopping cleanly

In all three modes, the daemon traps SIGINT and SIGTERM. On signal:

1. Cancels the root context, which propagates to the long-poll loop,
   in-flight handlers, and janitors.
2. Waits up to a few seconds for in-flight work to finish.
3. Flushes the log.
4. Exits 0.

If a handler is mid-screencapture or mid-curl when the signal arrives,
it gets cut off — the user sees no reply, but the next `macontrol`
start picks up cleanly. Telegram's update offset means the unhandled
update isn't re-delivered (acceptable for a request-response bot).
