# 🛠 Tools

Grab-bag: clipboard, timezone, NTP sync, disk list, and a Shortcuts
runner. The Shortcuts runner is the "I need something macontrol doesn't
have" escape hatch.

## Dashboard

```text
🛠 Tools

[ 📋 Clipboard (read) ] [ 📋 Clipboard (set)… ]
[ 🧭 Timezone…         ] [ 🔄 Sync time         ]
[ 💿 Disks              ]
[ ⚡ Run Shortcut…       ]    (only on macOS 13+)
[          🏠 Home        ]
```

## Buttons and flows

### 📋 Clipboard (read)

Panel:

```text
📋 Clipboard
```
```text
Whatever your clipboard currently contains.
```

Content over 3500 characters is truncated with `…(truncated)`.

Backing: `pbpaste`.

### 📋 Clipboard (set)… (flow)

```text
Bot: Send the text you want on the clipboard. /cancel to abort.
You: https://example.com
Bot: ✅ Clipboard updated.
```

Backing: `osascript -e 'set the clipboard to "<escaped text>"'`. Quotes
and backslashes in the input are escaped so arbitrary text works.

Use cases: grabbing something from your phone to paste on the Mac.

### 🧭 Timezone… (flow)

```text
Bot: Send the timezone string (e.g. Europe/Istanbul or UTC).
     /cancel to abort.
You: Europe/Istanbul
Bot: ✅ Timezone set — Europe/Istanbul
```

Accepts any IANA timezone name. Common ones:

```text
UTC
America/New_York
America/Los_Angeles
Europe/London
Europe/Istanbul
Asia/Tokyo
Asia/Dubai
Australia/Sydney
```

Full list: `sudo systemsetup -listtimezones` on a Mac, or the
`zoneinfo` directory (`/usr/share/zoneinfo/`).

Backing: `sudo systemsetup -settimezone <tz>`. **Requires** the narrow
sudoers entry.

### 🔄 Sync time

One-tap NTP sync:

```text
🛠 Tools

_Clock synced._
```

Backing: `sudo sntp -sS time.apple.com`. **Requires** the narrow
sudoers entry.

Use this if you suspect your Mac's clock is drifting (which shouldn't
normally happen, but can on long-running Macs that never sleep with
the network off).

### 💿 Disks

Panel:

```text
💿 Disks
```
```text
FS            Size     Used     Cap  Mount
/dev/disk1s1  228Gi    140Gi    64%  /
/dev/disk2s1  500Gi    300Gi    60%  /Volumes/External
```

Filters out devfs, VM mounts, and system-only paths. Shows user-facing
volumes (root, external drives, mounted DMGs).

Backing: `df -h`.

### ⚡ Run Shortcut… (macOS 13+, flow)

Hidden on macOS 11/12. On 13+:

```text
Bot: Send the Shortcut name (case-sensitive). /cancel to abort.
You: Turn on DND
Bot: ✅ Ran Turn on DND.
```

Names are exactly as they appear in the macOS Shortcuts app. Case and
spaces matter.

Backing: `shortcuts run "<name>"`. Whatever the Shortcut does, it does.
macontrol passes no input and doesn't read output.

Use cases:

- **Focus / Do Not Disturb toggles** — macOS doesn't expose Focus via
  CLI; a Shortcut that calls "Set Focus" bridges the gap.
- **HomeKit scene control** — "Good Night" Shortcut that dims lights.
- **iCloud flows** — "Archive today's photos" kind of thing.
- **Anything else** — if you can author it in Shortcuts, macontrol
  can call it.

### 🏠 Home

Edits to the inline home grid.

## What's backing this

| Action | Command |
|---|---|
| Clipboard read | `pbpaste` |
| Clipboard set | `osascript -e 'set the clipboard to "<text>"'` |
| Timezone current | `sudo systemsetup -gettimezone` |
| Timezone list | `sudo systemsetup -listtimezones` |
| Timezone set | `sudo systemsetup -settimezone <tz>` |
| Time sync | `sudo sntp -sS time.apple.com` |
| Disks | `df -h` |
| Shortcut list | `shortcuts list` |
| Shortcut run | `shortcuts run "<name>"` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#tools).

## Edge cases

### Clipboard with binary data

`pbpaste` returns binary data as-is, which Telegram will truncate or
reject when sent as a text message. Copying an image and tapping
Clipboard (read) gives an error or gibberish — macontrol doesn't try
to detect image clipboard content and attach it as a photo.

### Timezone unrecognized

Invalid IANA names return an error from `systemsetup`:

```text
⚠ set timezone failed: invalid time zone
```

Fix: use exact IANA format (`Continent/City`). `US/Pacific` works;
`PST` doesn't.

### `sudo -n systemsetup` prompt fails

Without the sudoers entry installed, Timezone and Sync time actions
fail with:

```text
⚠ tools — sntp failed
⚠ sudo: a password is required
```

**Fix**: install the narrow sudoers entry via `macontrol setup
--reconfigure` or manually per
[Permissions → Sudoers](../../permissions/sudoers.md).

### Shortcut not found

```text
⚠ run failed: exit status 1
```

`shortcuts run` is terse on failure. Double-check:

- Shortcut name case matches exactly (macOS Shortcuts is case-sensitive)
- The Shortcut is in the user's library, not iCloud-only (try opening
  the Shortcuts app and running it manually first)

### Shortcut takes a long time

macontrol's default subprocess timeout is 15 seconds. Long-running
Shortcuts will be killed at 15 s with a timeout error. To extend,
patch `internal/runner/runner.go` (`DefaultTimeout`) — or better,
design the Shortcut to fork-and-forget.

## Version gates

| Feature | Min macOS |
|---|---|
| Clipboard, Disks | 11.0 |
| Timezone, Sync time | 11.0 (needs sudoers for sudo) |
| Shortcut runner | 13.0 (Ventura) |
