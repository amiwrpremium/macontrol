# Common issues

Quick lookup table organized by symptom. For TCC- and sudoers-specific
errors see [Permission issues](permission-issues.md). For Bot API HTTP
codes see [Telegram errors](telegram-errors.md).

## Startup

### "command not found: macontrol"

The binary isn't on `$PATH`.

- **Brew install**: `/opt/homebrew/bin/macontrol`. If that's not on
  PATH, add `/opt/homebrew/bin` (Homebrew's installer normally does
  this).
- **Manual install via curl**: `/usr/local/bin/macontrol` or
  `~/.local/bin/macontrol`. The latter often isn't on PATH —
  `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc`.
- **Built from source**: `dist/macontrol` in the repo. Move it onto
  PATH or use the absolute path.

```bash
which macontrol      # should print the path
ls -la $(which macontrol)
```

### "macontrol: missing TELEGRAM_BOT_TOKEN in Keychain" / "missing ALLOWED_USER_IDS in Keychain"

The Keychain doesn't have the entries. Run:

```bash
macontrol setup
```

If you've already run setup, confirm both entries are present:

```bash
security find-generic-password -s com.amiwrpremium.macontrol -a $USER -w
security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a $USER -w
```

The first should print the bot token; the second a comma-separated
list of integer user IDs. If either prompts you for permission with
"macontrol wants to access your keychain", click **Always Allow**.

If either entry is missing, re-run `macontrol setup` (or
`macontrol token set` / `macontrol whitelist add` to add just one).

### Daemon crashes immediately after starting

```bash
launchctl list | grep macontrol
# If the second column shows non-zero (e.g. "1"), it crashed.
```

Check the log for the panic or fatal:

```bash
tail -50 ~/Library/Logs/macontrol/macontrol.log
tail -50 ~/Library/Logs/macontrol/macontrol.err.log
```

Common causes:

- **Invalid bot token** — `bot.New: Unauthorized`. Run `macontrol
  token set` and paste the correct token.
- **Invalid whitelist value** — `keychain whitelist value: invalid
  user id "abc"`. The Keychain entry's content must be all integers
  comma-separated. Re-run `macontrol setup --reconfigure` or fix
  with `macontrol whitelist add/remove`.
- **Log path not writable** — `permission denied: ~/Library/Logs/macontrol/`.
  Create the directory or fix permissions, or pass `--log-file=` to
  redirect to stderr.

### "bot exited" loop

The daemon starts, exits with an error, launchd restarts it, exits
again. Look at:

```bash
tail -200 ~/Library/Logs/macontrol/macontrol.log
```

The earliest `ERROR` message is usually the root cause. Subsequent
errors are downstream effects.

If it's a network error (`telegram transport error`), check that
your Mac can reach `api.telegram.org`:

```bash
TOKEN=$(security find-generic-password -s com.amiwrpremium.macontrol -a $USER -w)
curl -s "https://api.telegram.org/bot${TOKEN}/getMe"
```

Should return `{"ok":true,...}`. If you get a network error, check
DNS, firewall, VPN, captive-portal status.

## Bot doesn't reply

### Sent `/start`, no response

In order, check:

1. **Daemon running**:
   ```bash
   launchctl list | grep macontrol
   # PID column is a number (not "-")
   ```

2. **You're on the whitelist**:
   ```bash
   macontrol whitelist list
   ```
   Your numeric Telegram ID should appear. Get it from
   [@userinfobot](https://t.me/userinfobot) if unsure, then add with
   `macontrol whitelist add <id>`.

3. **You messaged the bot first** — Telegram doesn't let bots
   initiate; you have to send any message first. If you've never
   messaged the bot, do that now.

4. **Boot ping arrived?** When the daemon starts, every whitelisted
   user gets `✅ macontrol is up`. If you didn't see one, the daemon
   isn't running (loop back to step 1).

5. **Check the log for rejection**:
   ```bash
   tail -50 ~/Library/Logs/macontrol/macontrol.log | grep "rejected update"
   ```
   If you see `rejected update from non-whitelisted user sender=YOUR-ID`,
   your ID isn't on the list — fix it and restart.

### Bot replies to `/start` but not to keyboard taps

Probably a network blip while the callback was in flight. Tap again.
If it's persistent:

- Check the log for callback errors. Look for `callback dispatch err=…`.
- Confirm you tapped on a fresh keyboard, not one from before a
  daemon restart (callback short-ids are wiped on restart).

### Bot acts on some buttons but not others

Most likely a missing brew dep — see [Operations → Doctor](../operations/doctor.md):

```bash
macontrol doctor
```

Look for `✗` next to any brew formula. The buttons that depend on
that formula return errors silently in the dashboard.

### "Session expired" on a Bluetooth or Wi-Fi pick

The shortmap entry for that device/network has expired (15-min TTL).
Refresh the dashboard (e.g. tap **🔵 Bluetooth → Paired devices**
again) to re-issue fresh short-ids.

## Buttons return errors

### "command not found: brightness" / "blueutil" / "imagesnap"

Missing brew formula. Install:

```bash
brew install brightness blueutil terminal-notifier smctemp imagesnap
```

Then `macontrol doctor` to verify.

### "permission denied" or "operation not permitted" on screencapture/imagesnap

TCC permission missing. See [Permission issues](permission-issues.md).

### "exit status 1" on `pkill -x caffeinate`

False alarm. `pkill -x` exits 1 when there's nothing to kill —
macontrol treats that as success. If you see this in the log it's
informational, not an error.

### Sudo prompts in the log

```text
sudo: a password is required
```

Means the narrow sudoers entry isn't installed. Run:

```bash
macontrol setup --reconfigure
# answer 'y' to the sudoers prompt
```

Or install manually per [Permissions → Sudoers](../permissions/sudoers.md).

## Network / Telegram

### Bot stops responding after a network change

Wi-Fi roam, VPN connect/disconnect, or sleep/wake breaks the
long-poll. Wait 10–60 seconds; the library reconnects automatically.

If it doesn't recover in 2 minutes, restart:

```bash
brew services restart macontrol
# or
macontrol service stop && macontrol service start
```

### "Conflict: terminated by other getUpdates request"

Another daemon (or a developer running `go run` somewhere) is also
polling for the same bot token. Telegram only allows one long-poller
per bot.

Find and stop the other one:

```bash
ps aux | grep macontrol
launchctl list | grep macontrol
```

### "Too Many Requests: retry after N"

Telegram rate-limited you. macontrol's library handles this by
backing off automatically. If you see it persistently, you're sending
too many requests — typically by holding a `+1` button down on a
client that fires repeats. Stop, wait, retry.

## Logs / debugging

### Log file doesn't exist

```bash
ls -la ~/Library/Logs/macontrol/
```

If empty, either:
- The daemon never started (check `launchctl list`).
- Log path is overridden by `--log-file` in the LaunchAgent plist's
  `ProgramArguments`.
- The log directory has wrong permissions.

```bash
mkdir -p ~/Library/Logs/macontrol/
```

### Log is huge / out of disk

`lumberjack` rotation should prevent this. If logs are filling disk:

- Check the daemon's `--log-level` flag in the plist — debug is verbose.
- Check that lumberjack didn't fail to rotate (rare).
- Manually trim: `>~/Library/Logs/macontrol/macontrol.log`. The
  daemon will continue writing to it.

### Want more detail in logs

Edit the LaunchAgent plist to add `--log-level=debug` to
`ProgramArguments`, then `brew services restart macontrol`. Switch
back to `info` after diagnosing.

For a one-shot debug session without touching the plist:

```bash
macontrol service stop
macontrol run --log-level=debug --log-file=
# Ctrl-C when done
macontrol service start
```

## After upgrading

### "Unknown subcommand X" after a version bump

Older subcommands removed. Check `macontrol help` for the current
list.

### Existing config doesn't work after upgrade

Read the [CHANGELOG](../../CHANGELOG.md) for breaking changes.
Pre-1.0 releases (`v0.x`) may break config across minor versions.

If you can't find what changed, downgrade:

```bash
brew install amiwrpremium/tap/macontrol@v0.1.0
```

Then file a bug.

## Setup wizard issues

### "token verification failed"

The wizard called Telegram's `getMe` and got an error. Causes:

- **Wrong token format** — must be `<digits>:<base64-ish>`.
- **Revoked token** — you (or BotFather) revoked it. Get a new one
  with `/token` in BotFather.
- **No internet** — the wizard can't reach `api.telegram.org`.

### "visudo check failed"

The narrow sudoers content didn't validate. Should never happen unless
you're on a wildly different macOS that's changed `visudo`'s syntax.
File a bug with the visudo error message.

### "could not derive config path"

The wizard couldn't determine your home directory. Set `$HOME`
explicitly:

```bash
HOME=/Users/you macontrol setup
```

## Still stuck

File a bug at <https://github.com/amiwrpremium/macontrol/issues> with:

1. `macontrol --version`
2. `macontrol doctor` output
3. Last 100 lines of the log (with user IDs redacted to `123456789`)
4. Steps to reproduce

See [Support](../../SUPPORT.md).
