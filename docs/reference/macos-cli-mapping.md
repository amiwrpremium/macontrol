# macOS CLI mapping

The canonical table of every macontrol feature and the macOS command
that backs it. Use this if you're trying to figure out exactly what
the daemon would invoke for a given action.

## Sound

| Action | Command |
|---|---|
| Get level | `osascript -e 'set v to output volume of (get volume settings) … return (v as text) & "," & (m as text)'` |
| Get muted | (same script as above, returned together) |
| Set level | `osascript -e "set volume output volume <N>"` |
| Mute | `osascript -e "set volume output muted true"` |
| Unmute | `osascript -e "set volume output muted false"` |
| Say (TTS) | `say <text>` |

No brew deps. No sudo.

## Display

| Action | Command |
|---|---|
| Get level | `brightness -l` (parses `display 0:` line) |
| Set level | `brightness <0.0-1.0>` (3-decimal float) |
| Screensaver | `open -a ScreenSaverEngine` |

Brew dep: `brightness`. Without it, Get returns `unknown` and Set
fails with "executable not found".

Built-in display only. External monitors not supported.

## Power

| Action | Command |
|---|---|
| Lock | `/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession -suspend` |
| Sleep | `pmset sleepnow` |
| Restart | `osascript -e 'tell application "System Events" to restart'` |
| Shutdown | `osascript -e 'tell application "System Events" to shut down'` |
| Logout | `osascript -e 'tell application "System Events" to log out'` |
| Keep-awake | `nohup caffeinate -d -t <seconds> >/dev/null 2>&1 &` (forks via `sh -c`) |
| Cancel keep-awake | `pkill -x caffeinate` |

Restart, Shutdown, and Logout use the AppleScript path so they don't
need sudo — same code path as the Apple-menu equivalents. They prompt
each running app for a "want to abort?" reply, which means unsaved
work blocks them (politely).

The `pkill -x caffeinate` exit-1 (no matches) is treated as success in
the runner.

## Battery

| Action | Command |
|---|---|
| Status (percent, state, time) | `pmset -g batt` (parsed by regex `(\d+)%;\s*([^;]+?)(?:;\s*([^;]*))?$`) |
| Health (cycle, condition, max capacity, wattage) | `system_profiler SPPowerDataType` (parsed line-by-line) |

No brew deps. No sudo for read.

Desktop Macs (no battery present) are detected via `Battery is not
present` or `No batteries available` in the pmset output.

## Wi-Fi

| Action | Command |
|---|---|
| Discover Wi-Fi interface | `networksetup -listallhardwareports` (find "Wi-Fi" or "AirPort" port) |
| Get power | `networksetup -getairportpower <iface>` (parse "On"/"Off") |
| Get SSID | `networksetup -getairportnetwork <iface>` (parse "Current Wi-Fi Network: …") |
| Set power | `networksetup -setairportpower <iface> on/off` |
| Join | `networksetup -setairportnetwork <iface> <ssid> [<password>]` |
| Diagnostics | `sudo wdutil info` |
| Set DNS (Cloudflare) | `networksetup -setdnsservers Wi-Fi 1.1.1.1 1.0.0.1` |
| Set DNS (Google) | `networksetup -setdnsservers Wi-Fi 8.8.8.8 8.8.4.4` |
| Reset DNS | `networksetup -setdnsservers Wi-Fi Empty` |
| Speed test | `networkQuality -v` (parses `Downlink capacity: N Mbps` and `Uplink capacity: N Mbps`) |

Sudo: `wdutil info` requires the narrow sudoers entry.

Version gates: `networkQuality` needs macOS 12+. Other commands work
on macOS 11+.

Scanning is **not** supported (Apple removed `airport -s` in
macOS 14.4 with no replacement).

## Bluetooth

| Action | Command |
|---|---|
| Get power | `blueutil -p` (returns `0` or `1`) |
| Set power | `blueutil --power 0/1` |
| List paired | `blueutil --paired --format json` |
| List connected | `blueutil --connected --format json` |
| Connect | `blueutil --connect <addr>` |
| Disconnect | `blueutil --disconnect <addr>` |

Brew dep: `blueutil`. Required for every action; without it, the
category is unusable.

## System

| Action | Command |
|---|---|
| OS name + version + build | `sw_vers` (parses ProductName, ProductVersion, BuildVersion) |
| Hostname | `hostname` |
| Hardware model | `sysctl -n hw.model` |
| Chip name | `sysctl -n machdep.cpu.brand_string` |
| RAM bytes | `sysctl -n hw.memsize` |
| Uptime | `uptime` |
| Total cores | `system_profiler SPHardwareDataType` (parses "Total Number of Cores") |
| Thermal pressure | `sudo powermetrics -n 1 -i 1000 --samplers thermal` (parses "Current pressure level: <Nominal\|Moderate\|Heavy\|Trapping\|Sleeping>") |
| CPU °C | `smctemp -c` |
| GPU °C | `smctemp -g` |
| Memory pressure | `memory_pressure` (full output captured) |
| VM stats | `vm_stat` (full output captured) |
| Phys mem summary | `top -l 1 -s 0` (parses PhysMem line) |
| CPU usage line | `top -l 1 -s 0` (parses CPU usage line) |
| Top-N by CPU | `ps -Ao pid,pcpu,pmem,comm -r` (parses N rows) |
| Kill by PID | `kill <pid>` (SIGTERM) |
| Kill by name | `killall <name>` |

Sudo: `powermetrics` needs the narrow sudoers entry.

Brew dep: `smctemp` (optional, for °C readings).

## Media

| Action | Command |
|---|---|
| Screenshot | `screencapture <file>` |
| Silent screenshot | `screencapture -x <file>` |
| Specific display | `screencapture -D <N> <file>` |
| Window | `screencapture -l <window-id> <file>` |
| Region | `screencapture -R x,y,w,h <file>` |
| Delayed | `screencapture -T <secs> <file>` |
| Recording | `screencapture -v -V <secs> <file>` (macOS 14+) |
| Webcam photo | `imagesnap -q -w 1 <file>` |

TCC: Screenshot, Recording → **Screen Recording**. Webcam → **Camera**.

Brew dep: `imagesnap` (for webcam).

## Notify

| Action | Command |
|---|---|
| Rich notification | `terminal-notifier -group macontrol -title <T> -message <B> [-sound default]` |
| Basic notification | `osascript -e 'display notification "<B>" with title "<T>" [sound name "<S>"]'` |
| Speak (TTS) | `say <text>` |

Brew dep: `terminal-notifier` (preferred). Falls back to `osascript`
if not installed.

## Tools

| Action | Command |
|---|---|
| Clipboard read | `pbpaste` |
| Clipboard write | `osascript -e 'set the clipboard to "<text>"'` (escape `"` and `\`) |
| Get current timezone | `sudo systemsetup -gettimezone` |
| List timezones | `sudo systemsetup -listtimezones` |
| Set timezone | `sudo systemsetup -settimezone <tz>` |
| Force NTP sync | `sudo sntp -sS time.apple.com` |
| List disks | `df -h` (filters out devfs, /System/Volumes/VM, /private*) |
| List shortcuts | `shortcuts list` |
| Run shortcut | `shortcuts run "<name>"` |

Sudo: `systemsetup` and `sntp` need the narrow sudoers entry.

Version gates: `shortcuts` CLI needs macOS 13+.

## Misc / not exposed via the bot

These commands are referenced but not currently surfaced as buttons.
Listed for completeness so future contributors know what's already
plumbed:

| Capability | Where it could go |
|---|---|
| `caffeinate` (with full flag set) | Could add `caffeinate -i` / `-u` variants |
| `defaults` for app preferences | Out of scope today |
| `diskutil eject` | Could add to Tools → Disks |
| `system_profiler SPUSBDataType` | Could add to Tools |
| Shortcuts pre-listing (pick from menu) | Could add to Tools → Shortcut flow |

Open issues for any of these.

## Sudoers-required summary

The narrow `/etc/sudoers.d/macontrol` entry covers:

- `/usr/bin/pmset`
- `/usr/sbin/shutdown`
- `/usr/bin/wdutil info`
- `/usr/bin/powermetrics`
- `/usr/sbin/systemsetup`

Plus `sntp` is invoked via `sudo` but doesn't appear in the entry —
it works only if the user already has a more-permissive sudoers
entry. **TODO**: add `sntp` to the entry.

See [Permissions → Sudoers](../permissions/sudoers.md).

## Brew dep summary

| Formula | Required for |
|---|---|
| `brightness` | Display read/set |
| `blueutil` | Bluetooth (entire category) |
| `terminal-notifier` | Rich notifications (osascript fallback if missing) |
| `smctemp` | CPU/GPU °C in System → Temperature (pressure works without it) |
| `imagesnap` | Webcam photo |

Install all at once:

```bash
brew install brightness blueutil terminal-notifier smctemp imagesnap
```

`macontrol doctor` reports which are missing.

## TCC summary

| Permission | Required for |
|---|---|
| Screen Recording | Screenshot, Recording (any media capture of the display) |
| Accessibility | App listing via `osascript`, F1/F2 brightness fallback |
| Camera | Webcam photo |
| Automation | `osascript` controlling System Events (Restart, Shutdown, Logout, app listing) |

See [Permissions → TCC](../permissions/tcc.md).
