# Capability catalog

Every feature in macontrol is backed by a specific macOS CLI (or brew
formula). Version gates are detected at startup via `sw_vers`; buttons for
unavailable features are hidden or marked unavailable.

## Sound (§ `snd`)
`osascript` for volume/mute state, `say` for TTS.

## Display (§ `dsp`)
`brightness` (brew) for exact levels, `osascript` F1/F2 key codes as fallback.
`open -a ScreenSaverEngine` for the saver.

## Power (§ `pwr`)
`pmset sleepnow`, `CGSession -suspend` for lock, `osascript` for
restart/shutdown/logout (no sudo needed), `caffeinate -d -t` for keep-awake.
Destructive actions require a confirm tap.

## Wi-Fi (§ `wif`)
`networksetup` for power/join/DNS, `wdutil info` (sudo) for diagnostics,
`networkQuality` for the built-in speed test (macOS 12+). Scanning is
intentionally unsupported — the `airport` CLI was removed in macOS 14.4 and
has no official replacement.

## Bluetooth (§ `bt`)
`blueutil` (brew). Requires the brew formula; `macontrol doctor` flags it as
missing.

## Battery (§ `bat`)
`pmset -g batt` for current state, `system_profiler SPPowerDataType` for
health (cycle count, condition, max capacity).

## System (§ `sys`)
`sw_vers`, `sysctl`, `system_profiler` for info; `powermetrics --samplers
thermal` (sudo) for thermal pressure, `smctemp` (brew) for °C; `top`,
`vm_stat`, `memory_pressure`, `ps -r` for CPU/mem/processes; `kill`,
`killall` for termination.

## Media (§ `med`)
`screencapture` for stills and recording (Screen Recording TCC), `imagesnap`
(brew) for webcam photos (Camera TCC).

## Notify (§ `ntf`)
`terminal-notifier` (brew, preferred) with `osascript display notification`
fallback. `say` for TTS.

## Tools (§ `tls`)
`pbpaste`/`osascript set the clipboard` for clipboard; `systemsetup` (sudo)
for timezone; `sntp` (sudo) for clock sync; `df -h` for disks;
`shortcuts run` (macOS 13+) to run any Apple Shortcut you've authored.

## Version gates

| Feature | Min macOS |
|---|---|
| Everything above except below | 11.0 (Big Sur) |
| `networkQuality` speedtest | 12.0 (Monterey) |
| `shortcuts` CLI | 13.0 (Ventura) |

`macontrol doctor` prints the detected version and which features are gated.
