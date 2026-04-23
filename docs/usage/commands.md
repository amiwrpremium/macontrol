# Slash commands

macontrol has six slash commands plus two muscle-memory shortcuts.
Everything else is keyboard-driven — see
[Categories](categories/README.md).

## Table

| Command | Category | Description |
|---|---|---|
| `/start` | Entry | Send the home inline keyboard with all 10 categories |
| `/menu` | Entry | Same as `/start` — send the home inline keyboard |
| `/status` | Status | Dashboard snapshot (text + inline home grid) |
| `/help` | Help | Static help text with the command list |
| `/cancel` | Flow | Cancel any active multi-step flow |
| `/lock` | Shortcut | Lock the screen immediately |
| `/screenshot` | Shortcut | Silent full-screen screenshot |

`/start` and `/menu` are equivalent. `/start` exists because Telegram
clients show a "Start" button on first interaction that sends `/start`
automatically.

## /start and /menu

```text
/menu
```

Reply:

```text
🏠 macontrol

Pick a category below.
```

…with an inline keyboard showing all 10 categories attached to the
message.

Use when: first time, or whenever you want a fresh home grid (e.g.
after scrolling away from a previous one).

### Legacy reply-keyboard cleanup

Both `/start` and `/menu` (and the daemon's boot ping) also send a
throwaway clear-keyboard message that is immediately deleted. This
clears any persistent reply keyboard left over from pre-v0.1.4
clients (when macontrol still rendered a bottom-of-input button bar).
On modern installs you won't see anything — the throwaway flashes
and disappears within milliseconds.

## /status

```text
/status
```

Sends a single message with the aggregated state across info, battery,
and wifi services, plus an inline home grid for navigation:

```text
🖥 macontrol status

• macOS 15.3 on MacBookPro18,3 (tower.local)
• 🔋 78% · charging
• 📶 Wi-Fi on · SSID home
• ⏱ up 6 days, 3h 14m
```

The uptime line is parsed from `uptime` — Duration is rewritten as
`Xh Ym`. Tap **🖥 System → ℹ Info** for the full per-core load
breakdown.

Use when: you want a quick snapshot without navigating menus. Handy
for "is my Mac OK right now?" checks.

## /help

```text
/help
```

Returns a Markdown-formatted description of every slash command. Same
content as this table. Useful if you lost the home keyboard and don't
remember what's available.

## /cancel

```text
/cancel
```

Two cases:

- **If a flow is active** — the bot replies `✖ flow cancelled.` and the
  next message you send is treated normally (not as flow input).
- **If no flow is active** — the bot replies `🧹 nothing to cancel.`
  and nothing else happens.

Use when: you started a flow (e.g. tapped **Set exact value…**) and
changed your mind.

## /lock

```text
/lock
```

Puts the display to sleep via `pmset displaysleepnow`. No confirm. Whether the session actually locks depends on your "Require password after sleep" setting in System Settings → Privacy & Security.

Unlike the **⚡ Power → 🔒 Lock** button which also shows no confirm,
`/lock` has the advantage of working from any Telegram view — you
don't need to navigate to the Power category first. Good for quick
muscle-memory use.

Reply on success: `🔒 locked.`

## /screenshot

```text
/screenshot
```

Captures all displays silently (no shutter sound), returns a PNG as a
Telegram photo attached to your chat.

Requires **Screen Recording** TCC permission on first use — see
[Permissions → TCC](../permissions/tcc.md).

Unlike **📸 Media → Screenshot** which is also silent, `/screenshot`
is a one-liner — tap once and the photo lands. Good for hotkey-style
use.

## Commands Telegram shows in the attachment menu

If you ran the BotFather `/setcommands` step from
[Getting started → Telegram credentials](../getting-started/credentials-telegram.md#optional-botfather-tweaks),
Telegram clients display a tappable menu with your commands in the
attachment area (paperclip icon / `/` button). The menu is purely
cosmetic — it just types the command for you. You can always type
commands manually.

## What's *not* a slash command

Deliberate omissions — each of these could have been a command but
lives behind a keyboard instead because the keyboard UX is better for
it:

| Action | Where it lives | Why not a slash |
|---|---|---|
| Set exact volume | `/menu` → 🔊 Sound → Set exact value… | Needs a number; flow UX is better |
| Toggle Wi-Fi | `/menu` → 📶 Wi-Fi → Toggle | State-ful; the dashboard header shows current power |
| Join Wi-Fi | 🔗 Join network… | Two-step (SSID then password) |
| Kill a process | 🖥 System → Kill process… | Needs input |
| Restart / Shutdown | ⚡ Power → Restart/Shutdown (with confirm) | Destructive; confirm pattern better as a two-tap flow |
| Send a notification | 🔔 Notify → Send notification… | Two-field input |

## Adding more shortcuts

If you find yourself doing a particular action often and want a
one-command shortcut for it:

- **If the action already exists as a button**, add a slash wrapper in
  `internal/telegram/handlers/commands.go`. Pattern: case `"/myshortcut":
  return cmdMyShortcut(ctx, d, update)` — call the same domain function
  the button calls.
- **If the action is new**, see
  [Development → Adding a capability](../development/adding-a-capability.md).

PRs welcome, but keep the slash-command list tight — too many commands
defeats the keyboard-first UX.
