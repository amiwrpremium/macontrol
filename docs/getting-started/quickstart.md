# Quickstart

Assumes you've already [installed](installation.md) macontrol and have
your [Telegram credentials](credentials-telegram.md) ready (bot token +
your user ID).

Estimated time: 3–5 minutes.

## 1. Run the setup wizard

```bash
macontrol setup
```

The wizard walks through a fixed script:

```text
macontrol first-run setup. Press Ctrl-C to abort.

▸ Telegram bot token (from @BotFather): ************************
▸ Your Telegram user ID (from @userinfobot): 123456789
▸ Additional user IDs to allow, comma-separated (blank = none):

▸ Verifying token…  ✓ bot @amiwrpremium_macontrol_bot
▸ Writing config to /Users/you/Library/Application Support/macontrol/config.env  ✓
▸ Install LaunchAgent so macontrol starts at login? [Y/n] y
▸ LaunchAgent installed  ✓
▸ Install narrow sudoers entry (shutdown/pmset/wdutil/powermetrics/systemsetup)? [y/N] y
  Password:
▸ /etc/sudoers.d/macontrol written  ✓

TCC permissions to grant (System Settings → Privacy & Security):
  • Screen Recording  — /screenshot, /record
  • Accessibility     — app listing, fallback brightness
  • Camera            — /photo

▸ Start the daemon now? [Y/n] y
  daemon started.

Done. Send /start to @amiwrpremium_macontrol_bot.
```

### What each prompt does

| Prompt | What it does |
|---|---|
| Bot token | Hidden input (no echo). The wizard immediately validates it by calling Telegram's `getMe` endpoint. If the token is wrong you'll see `token verification failed` and the wizard exits. |
| Your user ID | First entry on the whitelist. Must be an integer. |
| Additional user IDs | Comma-separated extra IDs, or empty. |
| Install LaunchAgent? | Writes `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist` and `launchctl bootstrap`s it so the daemon starts at login and restarts on crash. Highly recommended. |
| Install sudoers entry? | Writes `/etc/sudoers.d/macontrol` with NOPASSWD access to only five binaries (`pmset`, `shutdown`, `wdutil info`, `powermetrics`, `systemsetup`). Enables the thermal and timezone features. Skip if you don't want `sudo` configured. |
| Start the daemon now? | Runs `launchctl bootstrap gui/$UID ...` to start the daemon immediately, without waiting for your next login. |

### Where things end up

| Thing | Where |
|---|---|
| Bot token | macOS Keychain — service `com.amiwrpremium.macontrol` |
| Whitelist | macOS Keychain — service `com.amiwrpremium.macontrol.whitelist` |
| Logs | `~/Library/Logs/macontrol/macontrol.log` (rotating) |
| LaunchAgent | `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist` |
| Sudoers (optional) | `/etc/sudoers.d/macontrol` |

No `.env` file. The wizard inserts both secrets directly into your
login keychain — encrypted at rest, ACL-restricted to the macontrol
binary, and inspectable via `security find-generic-password` if you're
curious. See [Configuration → File locations](../configuration/file-locations.md).

The first time the daemon reads from the Keychain, macOS may show a
one-time prompt asking you to grant access. Click **Always Allow** —
subsequent reads are silent.

## 2. Receive the boot ping

Within about five seconds of the daemon starting, every whitelisted
user gets a Telegram message from the bot:

```text
✅ macontrol is up

🖥 macontrol status

• macOS 15.3 on MacBookPro18,3 (tower.local)
• 🔋 78% · charging · 1:12 remaining
• 📶 Wi-Fi on · SSID home
• 10:22 up 6 days, 3:14, 4 users, load averages: 0.92 0.87 0.85

macOS 15.3 · 3/3 version-gated features available
```

If you don't see this within 30 seconds, the daemon didn't start — see
[Troubleshooting → Common issues](../troubleshooting/common-issues.md).

## 3. Open the home keyboard

In your bot's DM, send `/start` (or `/menu`). The bot replies with:

```text
🏠 macontrol

Pick a category below, or tap an inline button to dive into a dashboard.
```

…and below the input field, a **reply keyboard** appears with one
button per category:

```text
┌──────────────┬──────────────┬──────────────┐
│  🔊 Sound    │  💡 Display  │  🔋 Battery  │
├──────────────┼──────────────┼──────────────┤
│  📶 Wi-Fi    │  🔵 Bluetooth│  ⚡ Power    │
├──────────────┼──────────────┼──────────────┤
│  🖥 System   │  📸 Media    │  🔔 Notify   │
├──────────────┼──────────────┼──────────────┤
│  🛠 Tools    │  ❓ Help     │  ❌ Cancel   │
└──────────────┴──────────────┴──────────────┘
```

The keyboard is **one-shot**: tapping any button collapses the keyboard
and replaces it with an inline-keyboard dashboard for that category.
Send `/menu` again any time to re-summon it.

## 4. Tap a category

Try **🔊 Sound**. The bot replies with:

```text
🔊 Sound — 60% · unmuted
```

…and an **inline keyboard** under the message:

```text
┌──────┬──────┬──────┬──────┬──────┐
│  −5  │  −1  │ 🔇   │  +1  │  +5  │
├──────┴──────┴──────┴──────┴──────┤
│       Set exact value…          │
│       🔊 MAX (100)              │
├──────┬──────────────────────────┤
│ 🏠   │                          │
└──────┴──────────────────────────┘
```

Each button press:

- Calls the appropriate macOS command (`osascript` to set the volume).
- Edits the message you're looking at to show the new state.
- Keeps the keyboard visible so you can keep tapping.

## 5. Grant TCC permissions (first time)

Some actions trigger macOS privacy prompts the first time they run.
When you tap:

- **📸 Media → Screenshot** — macOS prompts for **Screen Recording**.
  Click *Open System Settings*, toggle `macontrol` on, then tap the
  button again in Telegram.
- **🖥 System → Running apps** (via `osascript`) — macOS prompts for
  **Accessibility**.
- **📸 Media → Webcam photo** — macOS prompts for **Camera**.

See [Permissions → TCC](../permissions/tcc.md) for the full list and
what each permission unlocks.

## Verify everything works

Checklist:

- [ ] Boot-ping message received in Telegram.
- [ ] `/status` returns the dashboard.
- [ ] `/menu` shows the home keyboard.
- [ ] Tapping a category shows its inline dashboard.
- [ ] Volume ± buttons actually change the Mac's volume.
- [ ] Non-whitelisted accounts get no reply when they DM the bot (try
      from a different Telegram account).

If any of these fail, see
[Troubleshooting → Common issues](../troubleshooting/common-issues.md).

## Next steps

- [First message](first-message.md) — a guided tour of every home
  category.
- [Usage → UX model](../usage/ux-model.md) — deeper explanation of
  keyboards, callbacks, and flows.
- [Usage → Categories](../usage/categories/README.md) — what every
  button in every dashboard does.
