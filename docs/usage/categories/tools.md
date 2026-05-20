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
[ ← Back ] [ 🏠 Home ]
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

### 🧭 Timezone…

Two-step picker. Tapping **🧭 Timezone…** shows the region picker
(IANA prefixes — Africa, America, Asia, Europe, Pacific, …):

```text
🧭 Set timezone  ·  Current: Europe/Istanbul

[ Africa (52) ]
[ America (149) ]
[ Antarctica (12) ]
[ Asia (78) ]
[ Atlantic (10) ]
[ Australia (12) ]
[ Europe (60) ]
[ Indian (10) ]
[ Pacific (30) ]
[ GMT ]
[ ⌨ Type exact name ]
[ 🔄 Refresh ] [ ← Back ]
[          🏠 Home          ]
```

Tap a region → paginated list of cities within it, with a flag emoji
per city (resolved from `/usr/share/zoneinfo/zone1970.tab`):

```text
🧭 America  ·  Page 1/10  ·  149 timezones  ·  Current: Europe/Istanbul

[ 🇺🇸 Adak ]
[ 🇺🇸 Anchorage ]
[ 🇦🇷 Argentina/Buenos_Aires ]
[ 🇦🇷 Argentina/Catamarca ]
…
[ ← Prev ] [ Next → ]
[ 🔍 Search ] [ ⌨ Type exact name ]
[ ← Back to regions ]
[          🏠 Home          ]
```

- **Tap a city** → applies the timezone immediately. The region
  picker re-renders with ``✅ Timezone set — `<tz>` `` above. No confirm.
- **🔍 Search** opens a one-step flow asking for a substring
  (case-insensitive, scoped to the current region). After you send
  it, the city list re-renders filtered. Prev/Next preserve the
  filter.
- **⌨ Type exact name** opens the original typed flow — useful if
  you know the exact IANA name and want to skip the picker:

  ```text
  Bot: Send the timezone string (e.g. Europe/Istanbul or UTC). /cancel.
  You: Europe/Istanbul
  Bot: ✅ Timezone set — Europe/Istanbul
  ```

Flag emojis come from the macOS-shipped `zone1970.tab` IANA →
ISO 3166-1 mapping. Multi-country zones (rare; e.g. some Russian
regions listed as `RU,UA`) take the first listed code. Antarctica,
GMT, UTC, and any zone not in the table render without a flag.

Backing: `sudo systemsetup -listtimezones` for the list,
`sudo systemsetup -gettimezone` for the current header,
`sudo systemsetup -settimezone <tz>` to apply. **Requires** the
narrow sudoers entry. Full timezone names live in a 15-min
server-side ShortMap so they fit Telegram's 64-byte callback_data
limit; leaving the dashboard open longer than that triggers
"session expired — refresh the timezone list".

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

Filtered to user-facing mounts only — `/` (root) and `/Volumes/*`
(external drives, mounted DMGs). Everything else (devfs,
`/System/Volumes/*`, `/Library/Developer/CoreSimulator/*`, etc.) is
hidden — it's noise for a remote-control bot.

Panel:

```text
💿 Disks

Tap a disk for actions.

[       /        · 460Gi · 54% used ]
[ /Volumes/Backup · 2Ti  · 38% used ]
[ 🔄 Refresh ] [ ← Back ]
[          🏠 Home          ]
```

Each row is a tappable button — drills into a per-disk page:

```text
💿 Macintosh HD — 494.4 GB total
Used: 17.9 GB · Free: 13.8 GB
FS: APFS · Device: /dev/disk3s1s1
Internal · Fixed · SSD · read-only

[ 📂 Open in Finder ] [ ⏏ Eject ]
[ 🔄 Refresh ] [ ← Back to Disks ]
[          🏠 Home              ]
```

- **📂 Open in Finder** runs `open <mount>` — reveals the volume in
  Finder on the Mac.
- **⏏ Eject** runs `diskutil eject <mount>`. **Only shown for
  removable volumes** (`Removable Media: Removable` per
  `diskutil info`). Root and other fixed disks don't get the
  button — accidentally ejecting `/` would be a bad day.
- After a successful eject, the disks list re-renders without the
  ejected volume.

Backing: `df -h` for the list, `diskutil info` for the per-disk
details, `diskutil eject` and `open` for the actions.

Mount paths are stored server-side in a 15-min TTL shortmap so long
`/Volumes/Foo Bar/` paths fit Telegram's 64-byte callback_data
limit. If you leave a disk dashboard open longer than 15 minutes and
tap a button, you'll get "session expired — refresh the disks list".

### ⚡ Run Shortcut… (macOS 13+, flow)

Hidden on macOS 11/12. On 13+, tap **⚡ Run Shortcut…** to get a
paginated list of every Shortcut from your Shortcuts.app:

```text
⚡ Run Shortcut  ·  Page 1/10  ·  142 shortcuts

[ Turn on DND                        ]
[ Toggle Wi-Fi                       ]
[ Send "Be right back" to last chat  ]
…  (15 entries per page)
[ ← Prev ] [ Next → ]
[ 🔍 Search ] [ ⌨ Type exact name ]
[ 🔄 Refresh ] [ ← Back ]
[          🏠 Home          ]
```

- **Tap a row** to run that Shortcut. The bot toasts
  `▶ Running '<name>'…` immediately and then re-renders the list
  with `✅ Ran <name>.` (or the error) above it.
- **🔍 Search** opens a flow that asks for a substring
  (case-insensitive). After you send it, the list re-renders with
  only matching shortcuts. Prev/Next preserve the filter.
- **⌨ Type exact name** opens the original flow — useful if you
  already know the exact Shortcut name and don't want to scroll:

  ```text
  Bot: Send the Shortcut name (case-sensitive). /cancel to abort.
  You: Turn on DND
  Bot: ✅ Ran Turn on DND.
  ```

Names match exactly what you see above each tile in the Shortcuts.app
sidebar. Long names are truncated on the button to ~40 characters
with `…`; the full name is what the `shortcuts run` CLI receives.

Mount paths and shortcut names go through a 15-min server-side
shortmap so they fit Telegram's 64-byte callback_data limit. Leave
the dashboard open longer than that and you'll get
"session expired — refresh the list".

Backing: `shortcuts list` for the list, `shortcuts run "<name>"`
for the action. Whatever the Shortcut does, it does — macontrol
passes no input and doesn't read output.

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
