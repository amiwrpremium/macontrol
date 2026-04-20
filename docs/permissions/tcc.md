# TCC grants

Apple's Transparency, Consent, and Control (TCC) framework gates a
small set of "sensitive" capabilities behind interactive user prompts.
There's no way to grant them ahead of time — macOS only prompts when
the action is first attempted, and only once per app per category.

Four TCC categories matter for macontrol.

## Screen Recording

**Triggers**: `screencapture` invocations.

**macontrol features that need it**:

- `📸 Media → Screenshot` (and `Silent shot`)
- `📸 Media → Record…`
- `/screenshot` slash command

**What you'll see the first time**:

A macOS prompt:

> "macontrol" would like to record this computer's screen and audio.
> Granting access enables Screen Recording even when the app is in
> the background.
>
> [Don't Allow] [Open System Settings]

Click **Open System Settings**. The Screen Recording panel opens.
Toggle the slider next to `macontrol` on.

After toggling, **restart the daemon** — TCC state is read at process
start, not on every action:

```bash
brew services restart macontrol
# or
macontrol service stop && macontrol service start
```

Then re-tap the action in Telegram. It works now.

**If you clicked Don't Allow by mistake**:

System Settings → Privacy & Security → Screen Recording → click the
`+` and add `macontrol` from `/opt/homebrew/bin/` (Homebrew) or
`/usr/local/bin/` (manual install). Then toggle on, then restart the
daemon.

## Accessibility

**Triggers**: AppleScript that uses `System Events` to enumerate or
control other apps.

**macontrol features that need it**:

- Listing running apps via `osascript -e 'tell application "System
  Events" to get name of every process whose background only is false'`
- The osascript fallback for brightness key codes (when `brightness`
  CLI isn't installed)

**The prompt**:

> "macontrol" would like to control "System Events".
>
> Granting access enables this app to send keystrokes and use
> accessibility features to control your computer.
>
> [Don't Allow] [Allow]

Click **Allow**.

Same restart-the-daemon dance applies.

**To revoke**:

System Settings → Privacy & Security → Accessibility → uncheck
`macontrol`.

## Camera

**Triggers**: any process opening the FaceTime HD or Studio Display
camera.

**macontrol features that need it**:

- `📸 Media → Webcam photo` (via `imagesnap`)

**The prompt**:

> "macontrol" would like to access the camera.
>
> [Don't Allow] [OK]

The actual binary that triggers the prompt is `imagesnap`, not
`macontrol` itself. macOS sometimes shows the prompt as
"`imagesnap` would like…" instead. Either way, click **OK**.

**To revoke**:

System Settings → Privacy & Security → Camera → uncheck
`macontrol` (and `imagesnap` if it appears separately).

## Automation

**Triggers**: AppleScript that targets a specific app for control
(`tell application "Safari"`, `tell application "System Events"`).

**macontrol features that need it**:

- `⚡ Power → Restart / Shutdown / Logout` (via System Events)
- `📸 Media → Webcam photo` first invocation may also trigger this

**The prompt** (different from the others):

> "macontrol" wants access to control "System Events".
> Allowing control will provide access to documents and data in
> "System Events", and to perform actions within that app.
>
> [Don't Allow] [OK]

Click **OK** for each app the bot scripts (typically just System
Events).

**To revoke**:

System Settings → Privacy & Security → Automation → expand
`macontrol`, uncheck each app underneath.

## Why macOS asks per-app, per-category

TCC is per-app + per-category. Granting macontrol Screen Recording
doesn't grant it Camera; granting macontrol access to System Events
doesn't grant access to Safari. This is by design — minimum surface
area.

It also means every brew dep you install (`imagesnap`, `terminal-notifier`,
etc.) might trigger its own TCC prompts the first time it's invoked,
because TCC tracks the **executable that called the API**, not the
parent process. So you might see "imagesnap" in the Camera list rather
than "macontrol".

## Why some prompts attribute the wrong process

You may see prompts referring to "macontrol" by some other name —
typically the binary path. This happens because macontrol launches
sub-processes (`screencapture`, `imagesnap`) and **macOS attributes
the prompt to whichever binary called the API**.

Practically:

| Action | Prompt likely shows |
|---|---|
| Screenshot | `macontrol` (it owns the `screencapture` exec) |
| Recording | `macontrol` (same as above) |
| Webcam | `imagesnap` (because `imagesnap` itself does the camera open) |
| Restart | `macontrol` (because it sends the AppleScript directly) |

You'll grant separately to `macontrol` and `imagesnap` in the Camera
list.

## TCC database location

TCC entries live in:

```text
/Library/Application Support/com.apple.TCC/TCC.db   # system-wide
~/Library/Application Support/com.apple.TCC/TCC.db  # per-user (you)
```

Don't edit these files manually. Use System Settings → Privacy &
Security as the only safe interface.

## Resetting all macontrol TCC grants

Useful for diagnosing weirdness or before uninstalling:

```bash
tccutil reset All com.amiwrpremium.macontrol
```

Or per-category:

```bash
tccutil reset ScreenCapture
tccutil reset Accessibility
tccutil reset Camera
tccutil reset AppleEvents      # Automation
```

Note `tccutil reset` is **broad** — it clears state for *all* apps in
that category, not just macontrol. Use sparingly.

## What about macOS-managed Macs (MDM)

If your Mac is enrolled in a corporate MDM, your IT department may
have pushed a "Privacy Preferences Policy Control" (PPPC) profile that
either:

- **Pre-grants** the permissions (you don't see the prompt — it's
  already on)
- **Pre-denies** them and locks the toggle (you can't enable it)

If the toggles are greyed out in System Settings, this is why. Your IT
department needs to update the PPPC profile to allow `macontrol`. Open
an issue at the macontrol repo if you need a sample profile to share
with them.
