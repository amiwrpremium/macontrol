# First message: touring the home keyboard

A guided walk through what each home-keyboard category does, with the
single-button "hello world" for each. Assumes you've completed the
[Quickstart](quickstart.md) and the bot is responding.

## The home keyboard

Send `/start` or `/menu`. You'll see 10 category buttons, plus Help
and Cancel. Categories fall into two shapes:

- **State-ful** dashboards (Sound, Display, Battery, Wi-Fi, Bluetooth)
  — the message text shows current state, buttons change it, the
  message edits in place.
- **Action menus** (Power, System, Media, Notify, Tools) — the message
  is a list of actions to pick from.

## 🔊 Sound

Tap it. You see:

```text
🔊 Sound — 60% · unmuted
```

Tap **+1**. The message edits to:

```text
🔊 Sound — 61% · unmuted
```

Try **MUTE**. The button changes to 🔈 Unmute and your Mac goes silent.

Try **Set exact value…** — the bot sends "Enter target volume (0-100)"
and waits for your reply. Send `42`. The volume goes to 42, the flow
ends, the message says "✅ Volume set — 42% · muted: false".

## 💡 Display

Tap it. If you have `brew install brightness` installed you'll see a
level like `70%`; otherwise `unknown` with a note.

Tap **−10** to dim. Tap **Set exact value…** and send `80` to set it.

**🌙 Screensaver** starts macOS's screensaver immediately. Move the
mouse to dismiss.

## 🔋 Battery

Tap it. On a laptop:

```text
⚡ Battery — 78% · charging · 1:12 remaining
```

Tap **📊 Health** to see cycle count, condition, and max capacity.

On a desktop Mac (no battery):

```text
🔋 Battery — not present (desktop Mac)
```

## 📶 Wi-Fi

Tap it. You see current power state and SSID:

```text
📶 Wi-Fi — on · SSID home · iface en0
```

Try **🌐 DNS → Cloudflare** to switch DNS to 1.1.1.1 / 1.0.0.1.

Try **⚡ Speed test** (requires macOS 12+) — takes about 15 seconds, then
posts:

```text
⚡ Speedtest

• Down: 523.4 Mbps
• Up:   87.2 Mbps
```

Try **🔗 Join network…** — the bot prompts for the SSID, then the
password. (Send `-` as the password for open networks.)

## 🔵 Bluetooth

Requires `brew install blueutil`. Without it, tapping shows an error.

With it installed:

```text
🔵 Bluetooth — on
```

Tap **📋 Paired devices** to get a list of every paired device, each
with a 🔗 or ✂ button depending on connection state. Tapping toggles.

## ⚡ Power

Simple menu:

```text
⚡ Power

Tap an action. Destructive actions require a second tap to confirm.
```

Try **🔒 Lock** — the screen locks immediately, no confirm.

Try **🔁 Restart** — the bot replies with a Confirm / Cancel sub-keyboard.
Tap **✖ Cancel** (you don't actually want to reboot right now).

Try **☕ Keep awake…** — the bot asks for a duration in minutes, then
runs `caffeinate -d -t <seconds>`.

## 🖥 System

Action menu with read-only info and one destructive action:

- **ℹ Info** → OS + hardware summary
- **🌡 Temperature** → thermal pressure (Nominal / Moderate / Heavy)
  and, if `smctemp` is installed, CPU and GPU °C
- **🧠 Memory** → memory pressure summary
- **⚙ CPU** → load average and top CPU header
- **📋 Top 10 processes** → PID / %CPU / %MEM / command
- **🔪 Kill process…** → starts a flow; you send a PID or a name

## 📸 Media

**📷 Screenshot** captures all displays. First time: macOS prompts for
**Screen Recording** permission. Grant it, then tap again. The bot
replies with a PNG as a Telegram photo.

**📹 Record…** asks for a duration (1–120 seconds) and returns a MOV.

**📸 Webcam photo** takes a single frame from the built-in FaceTime
camera. Requires `brew install imagesnap` and **Camera** permission.

## 🔔 Notify

**✉ Send notification…** — flow asks for `title | body` (or just a
body); bot replies "Notified via terminal-notifier" or "Notified via
osascript" depending on which transport was used.

**🗣 Say…** — flow asks for text, Mac's TTS speaks it.

## 🛠 Tools

Grab-bag:

- **📋 Clipboard (read)** — shows current clipboard contents.
- **📋 Clipboard (set)…** — flow asks for text, writes to clipboard.
- **🧭 Timezone…** — flow asks for an IANA timezone name (e.g.
  `Europe/Istanbul`), sets it via `sudo systemsetup`.
- **🔄 Sync time** — forces an NTP sync.
- **💿 Disks** — `df -h` filtered to user-facing mounts.
- **⚡ Run Shortcut…** (macOS 13+) — flow asks for a Shortcut name
  (case-sensitive), runs it via `shortcuts run`.

## Navigating between dashboards

Every inline keyboard has a **🏠 Home** button in the last row. Tapping
it edits the message back to the inline home grid, from which you can
enter any category again.

## What to do next

- Read [Usage → UX model](../usage/ux-model.md) to understand why the
  keyboards behave the way they do.
- Read [Usage → Commands](../usage/commands.md) for the slash commands
  that complement the keyboard UX.
- Read [Usage → Categories](../usage/categories/README.md) for every
  button with its caveats and edge cases.

If a button does nothing or returns an error, check
[Troubleshooting → Permission issues](../troubleshooting/permission-issues.md)
first — most mysteries are TCC prompts you missed.
