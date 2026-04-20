# Permission issues

The two layers of permissions (TCC and sudoers) fail in distinctive
ways. This doc maps the most common error messages to root causes.

For background: [Permissions → README](../permissions/README.md).

## TCC — Screen Recording

### Symptom: screenshot returns a black or empty image

You tapped **📸 Media → Screenshot** and got a PNG that's all black,
or that just shows the menu bar with everything else blank.

**Cause**: `macontrol` doesn't have Screen Recording TCC. macOS
silently produces blank captures rather than failing.

**Fix**:

1. System Settings → Privacy & Security → Screen Recording.
2. If `macontrol` is in the list, toggle it on. If not, click the `+`
   and add it from `/opt/homebrew/bin/macontrol` (Homebrew) or
   `/usr/local/bin/macontrol` (manual install).
3. Restart the daemon: `brew services restart macontrol`.
4. Re-tap the screenshot button.

### Symptom: log shows `screencapture: cannot run because of TCC`

Newer macOS versions emit this directly. Same fix as above.

### Symptom: TCC prompt never appears

You tapped the screenshot button and macOS didn't ask. Either:

- You already granted (and forgot) — check System Settings → Screen
  Recording.
- You already denied (and forgot) — same place; toggle on.
- Your Mac is MDM-managed and the prompt is suppressed by a PPPC
  profile. Talk to your IT department.

## TCC — Camera

### Symptom: webcam button returns "operation not permitted"

`imagesnap` couldn't open the camera.

**Cause**: Camera TCC missing for `imagesnap` (or for `macontrol` if
the prompt was attributed to it).

**Fix**:

1. System Settings → Privacy & Security → Camera.
2. Look for both `macontrol` AND `imagesnap`. Toggle on each that
   appears.
3. If neither is in the list, run the action once to trigger the
   prompt; click Allow.
4. Restart the daemon.

### Symptom: webcam works in other apps but not macontrol

The TCC entry is per-binary. If FaceTime or Photo Booth has Camera
permission but `imagesnap`/`macontrol` doesn't, that's expected — they
each need a separate grant.

## TCC — Accessibility

### Symptom: app listing returns "operation not permitted"

The Top-N processes list calls `osascript` to enumerate running apps,
which needs Accessibility.

**Fix**: System Settings → Privacy & Security → Accessibility →
toggle `macontrol` on. Restart the daemon.

### Symptom: brightness key-codes don't work

The osascript fallback for brightness uses key codes, which needs
Accessibility. Without `brightness` brew formula AND without
Accessibility, the dashboard shows "unknown" and buttons silently fail.

**Fix**: install `brew install brightness` (preferred — no TCC
needed), or grant Accessibility to use the fallback.

## TCC — Automation

### Symptom: Restart/Shutdown/Logout buttons return cryptic errors

```text
osascript: System Events got an error: macontrol is not allowed to send keystrokes.
```

…or:

```text
osascript: System Events got an error: An error of type -10810 occurred.
```

**Cause**: Automation permission missing for `macontrol` to control
System Events.

**Fix**:

1. System Settings → Privacy & Security → Automation.
2. Find `macontrol`, expand it.
3. Toggle System Events on.
4. Tap the action again — this time it works.

If `macontrol` doesn't appear under Automation:

- Tap the action once to trigger the prompt; click OK.
- The entry should appear after that.

## Sudoers — narrow entry

### Symptom: log shows "sudo: a password is required"

```text
WARN  msg="callback dispatch"  err="sudo: -n: a password is required"
```

**Cause**: a sudo-needing action ran (e.g. powermetrics, wdutil info,
systemsetup) but the narrow sudoers entry isn't installed.

**Affected actions**:

- 🌡 System → Temperature (powermetrics)
- 📶 Wi-Fi → Info (wdutil)
- 🛠 Tools → Timezone… (systemsetup)
- 🛠 Tools → Sync time (sntp)

**Fix**:

```bash
macontrol setup --reconfigure
# answer 'y' to the sudoers prompt
```

Or install manually per [Permissions → Sudoers](../permissions/sudoers.md).

Then verify:

```bash
sudo -n pmset -g
# Should print without prompting; if it prompts, the entry isn't active.
```

### Symptom: sudoers entry exists but sudo still prompts

Possible causes:

1. **File permissions wrong**: `sudo` ignores files in `/etc/sudoers.d/`
   that aren't owned by `root:wheel` with mode `0440`.
   ```bash
   ls -la /etc/sudoers.d/macontrol
   # Expected: -r--r-----  1 root  wheel  ... macontrol
   ```
   Fix:
   ```bash
   sudo chown root:wheel /etc/sudoers.d/macontrol
   sudo chmod 0440 /etc/sudoers.d/macontrol
   ```

2. **Username mismatch**: the entry references your old username.
   Check:
   ```bash
   sudo cat /etc/sudoers.d/macontrol
   whoami
   ```
   If they differ, regenerate via `macontrol setup --reconfigure`.

3. **Syntax error silently disabling the file**: macOS sudo skips
   sudoers files with parse errors. Validate:
   ```bash
   sudo visudo -cf /etc/sudoers.d/macontrol
   # Expected: parsed OK
   ```

### Symptom: `macontrol setup` says "visudo check failed"

The wizard validates the generated sudoers content with `visudo -cf`
before installing. If it fails:

- macOS may have updated visudo's syntax requirements (rare).
- Your username may contain characters sudo doesn't accept (very
  rare).

Capture the visudo error:

```bash
SUDOERS_DEBUG=1 macontrol setup --reconfigure
```

Then file a bug with the output.

## Other

### "context deadline exceeded" on subprocess calls

A macOS CLI took longer than the default 15-second runner timeout.
Common with:

- **Speed test** on a slow connection.
- **system_profiler** on a Mac with many devices.
- **shortcuts run** for slow Shortcuts.

The runner kills the subprocess after 15 s. The handler reports the
timeout to the user.

**Fix options**:

- For the speed test: nothing — `networkQuality` is usually under 15 s.
  If your network is too slow, the test legitimately doesn't fit.
- For Shortcuts: design the Shortcut to fork-and-forget so it returns
  quickly.
- For longer-running needs: patch `internal/runner/runner.go` —
  `DefaultTimeout` — and rebuild. Or wrap the action with a
  per-handler longer context.

### "TCC permission revoked" after macOS upgrade

macOS upgrades sometimes reset TCC entries for apps. After a major
upgrade, re-grant any permissions that stopped working.

### Permissions work in foreground but not under launchd

Possible causes:

- **launchd PATH** doesn't include the directory with the brew binary.
  The plist's `EnvironmentVariables` block sets PATH explicitly to
  `/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`. If
  it's missing or wrong, brew binaries aren't found.

- **TCC attribution**: TCC tracks the executing binary. If macontrol
  was first granted under one path (e.g. `/usr/local/bin/macontrol`)
  and the launchd plist now invokes a different path (e.g.
  `/opt/homebrew/bin/macontrol`), TCC sees them as different binaries.
  Grant the new path.

  ```bash
  # Confirm what launchd is running
  launchctl print gui/$UID/com.amiwrpremium.macontrol | grep -A1 'program ='
  ```

  If the path differs from where you've granted TCC, add the new path
  to the relevant TCC categories.

### TCC reset

Nuclear option for clearing all macontrol TCC state:

```bash
tccutil reset All com.amiwrpremium.macontrol
```

You'll get fresh prompts for everything. Use only if grants are
severely tangled.
