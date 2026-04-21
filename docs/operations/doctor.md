# Doctor

`macontrol doctor` prints a one-shot health report. Run it any time
something doesn't seem right; output should be enough to diagnose 80%
of issues.

## Running it

```bash
macontrol doctor
```

Takes about 1–2 seconds. Doesn't talk to Telegram, doesn't need the
daemon to be running.

## Sample output

On a healthy ASi Mac with all brew deps installed:

```text
macontrol v0.1.0 (abc1234, 2026-04-20)
runtime: darwin/arm64

macOS:            15.3
networkQuality:   true
shortcuts CLI:    true
wdutil info:      true

brew deps (auto-installed when macontrol is installed via Homebrew):
  ✓ brightness         💡 brightness control
     └ brew install brightness
  ✓ blueutil           🔵 Bluetooth toggle/list/connect
     └ brew install blueutil
  ✓ terminal-notifier  🔔 rich notifications (fallback: osascript)
     └ brew install terminal-notifier
  ✓ smctemp            🌡 °C readings on Apple Silicon
     └ brew install narugit/tap/smctemp
  ✓ imagesnap          📸 webcam photos
     └ brew install imagesnap

sudoers (pmset):
  ✓ `sudo -n pmset -g` works

keychain:
  ✓ bot token present in Keychain (com.amiwrpremium.macontrol)
  ✓ whitelist present in Keychain (com.amiwrpremium.macontrol.whitelist)
```

On a manual install (no brew) with nothing else installed:

```text
macontrol v0.1.0 (abc1234, 2026-04-20)
runtime: darwin/arm64

macOS:            15.3
networkQuality:   true
shortcuts CLI:    true
wdutil info:      true

brew deps (auto-installed when macontrol is installed via Homebrew):
  ✗ brightness         💡 brightness control
     └ brew install brightness
  ✗ blueutil           🔵 Bluetooth toggle/list/connect
     └ brew install blueutil
  ✗ terminal-notifier  🔔 rich notifications (fallback: osascript)
     └ brew install terminal-notifier
  ✗ smctemp            🌡 °C readings on Apple Silicon
     └ brew install narugit/tap/smctemp
  ✗ imagesnap          📸 webcam photos
     └ brew install imagesnap

sudoers (pmset):
  ✗ `sudo -n pmset -g` failed — install sudoers entry via `macontrol setup`

keychain:
  ✗ bot token missing from Keychain — run `macontrol setup` (or `macontrol token set`/`whitelist add`)
  ✗ whitelist missing from Keychain — run `macontrol setup` (or `macontrol token set`/`whitelist add`)
```

## What each line means

### Header

```text
macontrol v0.1.0 (abc1234, 2026-04-20)
runtime: darwin/arm64
```

- The first line is the binary's version, commit, and build date —
  injected at link time by GoReleaser. Local `go run` shows
  `v0.1.0-dev (none, unknown)`.
- The runtime line shows the host OS/arch. If you see anything other
  than `darwin/arm64`, you're running the wrong binary — macontrol
  won't function correctly.

### macOS section

```text
macOS:            15.3
networkQuality:   true
shortcuts CLI:    true
wdutil info:      true
```

- **macOS** version detected via `sw_vers -productVersion`.
- **networkQuality**: `true` if version ≥ 12.0. Required for the
  📶 Wi-Fi → Speed test button.
- **shortcuts CLI**: `true` if version ≥ 13.0. Required for the
  🛠 Tools → Run Shortcut… flow.
- **wdutil info**: `true` for all supported macOS (≥ 11.0). Required
  for 📶 Wi-Fi → Info.

If macOS detection fails (rare — would mean `sw_vers` is missing or
broken):

```text
⚠ could not run sw_vers: exec: "sw_vers": executable file not found in $PATH
```

This effectively means macontrol can't detect what features are
available, and most things will fail. Check that `/usr/bin/sw_vers`
exists.

### Optional brew deps

Each entry shows:

- `✓` or `✗` — whether the binary is on `$PATH`.
- The binary name and what it unlocks.
- The install command (`brew install <name>`).

Missing deps don't break macontrol — they just disable the related
buttons. You'll see "unavailable" messages in Telegram dashboards for
the affected features.

### Sudoers

```text
sudoers (pmset):
  ✓ `sudo -n pmset -g` works
```

`sudo -n` means "non-interactive" — fail rather than prompt for a
password. The check runs `sudo -n pmset -g` (a harmless read) and
checks the exit code:

- `✓` — the narrow sudoers entry is installed and active.
- `✗` — sudo prompted for a password (which non-interactive can't
  satisfy), so either no entry exists or the entry is broken.

If `✗`, install the entry via `macontrol setup` (which validates with
`visudo` first). See [Permissions → Sudoers](../permissions/sudoers.md).

## When doctor isn't enough

If doctor passes but a feature still doesn't work, the issue is
probably TCC. Doctor doesn't check TCC because there's no programmatic
way to query it without elevated privileges.

The fallback diagnostic path:

1. Check the log for the failing action's error message:
   `tail -f ~/Library/Logs/macontrol/macontrol.log`
2. Match the error against [Troubleshooting → Common issues](../troubleshooting/common-issues.md)
   or [Permission issues](../troubleshooting/permission-issues.md).

## Using doctor in CI / scripts

Exit code is always 0, even when checks fail. Doctor is a *diagnostic
report*, not an enforcement tool. Parse the output if you want to
script around it:

```bash
macontrol doctor | grep -E '^\s+✗' && echo "missing deps detected"
```

This is fine for local "check before installing" scripts. Don't use
it in production-critical flows; the format may change.
