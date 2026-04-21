# ⚡ Power

Lock, sleep, restart, shutdown, logout, and keep-awake control.
Destructive actions (restart, shutdown, logout) need a confirm tap.

## Dashboard

```text
⚡ Power

Tap an action. Destructive actions require a second tap to confirm.

[ 🔒 Lock        ] [ 💤 Sleep       ]
[ 🔁 Restart     ] [ ⏻ Shutdown    ] [ 🚪 Logout   ]
[ ☕ Keep awake… ] [ 🌑 Cancel awake ]
[          🏠 Home                   ]
```

## Buttons

### 🔒 Lock

No confirm. Puts the display to sleep via:

```bash
pmset displaysleepnow
```

Whether this also locks the session depends on **System Settings →
Privacy & Security → "Require password after sleep or screen saver
begins"**. Set that to *Immediately* (or a short delay) for the Lock
button to actually lock, which is the default on modern macOS.

Works on macOS 11 through 26 with no sudo and no Accessibility
permission. The legacy `User.menu/CGSession -suspend` path no longer
ships on current macOS.

The bot replies with a toast "Locking…" but doesn't edit the message.

Same effect as the `/lock` slash command.

### 💤 Sleep

No confirm. Puts the Mac to sleep via `pmset sleepnow`. The daemon
continues running and the bot will be offline until you wake the Mac.

### 🔁 Restart, ⏻ Shutdown, 🚪 Logout

These **do** confirm. First tap edits the message to:

```text
⚠ Confirm restart

This will affect your Mac immediately. Tap Confirm to proceed.

[ ✅ Confirm ] [ ✖ Cancel ]
```

- **Confirm** runs the action (AppleScript → `tell application "System
  Events" to restart` / `shut down` / `log out`).
- **Cancel** edits back to the inline home grid.

No timeout on the confirm. If you don't tap Confirm, nothing happens.

The AppleScript path doesn't require sudo — macOS allows the
current-user session to trigger these. (Why? Because it's the same
code path as the Apple menu's Restart/Shutdown/Logout commands.)

### ☕ Keep awake… (flow)

Prevents sleep for a specified duration. Useful for "don't sleep while
a long build runs".

```text
Bot: Keep awake for how many minutes? (1-1440). Reply /cancel to abort.
You: 30
Bot: ☕ Keep-awake running for 30 min.
```

Internally spawns `nohup caffeinate -d -t <seconds> &`. The `-d` flag
keeps the display on too. The `caffeinate` process forks into the
background; macontrol does not track its PID.

Max 1440 minutes (24 hours). Below 1 or above 1440 re-prompts without
cancelling.

### 🌑 Cancel awake

Kills any running `caffeinate` processes with `pkill -x caffeinate`.
If multiple keep-awakes are stacked, this kills all of them.

No-op if no caffeinate is running — the `pkill -x` exit 1 is treated
as success.

## What's backing this

| Action | Command |
|---|---|
| Lock | `pmset displaysleepnow` (no sudo; actual lock requires "Require password after sleep" in System Settings) |
| Sleep | `pmset sleepnow` (no sudo — technically reads a user agent, not setting system state) |
| Restart | `osascript -e 'tell application "System Events" to restart'` |
| Shutdown | `osascript -e 'tell application "System Events" to shut down'` |
| Logout | `osascript -e 'tell application "System Events" to log out'` |
| Keep-awake | `caffeinate -d -t <seconds>` |
| Cancel-awake | `pkill -x caffeinate` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#power).

## Edge cases

### AppleScript prompting for Automation permission

The first time you tap Restart / Shutdown / Logout, macOS may prompt
the `macontrol` binary for **Automation** permission to control System
Events. Click **Allow**. If you click Deny, future attempts silently
fail — you have to grant it manually in System Settings → Privacy &
Security → Automation. See [Permissions → TCC](../../permissions/tcc.md).

### Keep-awake outliving the daemon

`caffeinate` is spawned detached with `nohup`. If macontrol stops,
existing keep-awake timers keep running. To stop them, either wait
for them to expire or use **🌑 Cancel awake** (if macontrol is still
alive) or `sudo killall caffeinate` manually.

### Restart/Shutdown when you're running unsaved work

The AppleScript path is polite — macOS asks each running application
if it wants to abort the shutdown (same as the Apple menu). If any
app says "no" (e.g. unsaved document prompt), the shutdown is
aborted. There's no force-shutdown option in macontrol — use
`sudo shutdown -h now` via terminal if you really need it.

## Version gates

None — every Power action works on macOS 11+ without brew deps.
