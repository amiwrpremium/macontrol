# UX model

macontrol's interaction model has three distinct surfaces. Knowing
which one you're in tells you what the next button-press will do,
what collapses, and what edits.

## 1. Slash commands (entry points)

Six built-in slash commands; everything else is reachable from an
inline keyboard. See [Commands](commands.md) for the full list.

| Command | Effect |
|---|---|
| `/start`, `/menu` | Sends the home inline keyboard with all 10 categories |
| `/status` | Sends a one-message dashboard snapshot with the home grid attached |
| `/help` | Static help text |
| `/cancel` | Cancels any active flow for this chat |
| `/lock` | Locks the screen immediately (no confirm) |
| `/screenshot` | Sends a silent full-screen screenshot |

Slash commands trigger the same code paths as the keyboard
equivalents where they overlap (e.g. `/lock` calls the same domain
function as **⚡ Power → 🔒 Lock**).

The slash commands also appear in Telegram's command menu (the
`/` button next to the input field) for discoverability — set via
`@BotFather → /setcommands`.

## 2. Inline keyboards (edit-in-place dashboards)

```text
🔊 Sound — 60% · unmuted

[ −5 ] [ −1 ] [ MUTE ] [ +1 ] [ +5 ]
[       Set exact value…        ]
[          🔊 MAX (100)         ]
[             🏠 Home            ]
```

Inline keyboards are attached to a specific message. Properties:

- **Persistent within the message**: they don't collapse on tap.
- **Edit in place**: when you tap a button, the handler runs the
  underlying action and then **edits the same message** with the new
  state. No new message is sent.
- **Stateful**: the message text always reflects current state. Tap
  `+1` and the header instantly updates to `61% · unmuted`.

Every nested menu carries a `[← Back] [🏠 Home]` row at the bottom:

- **🏠 Home** edits the message back to the home grid (a 3×N inline
  keyboard with all 10 categories). Always present.
- **← Back** edits the message to the immediate parent. On a
  category dashboard (one level deep), the parent IS the home grid,
  so Back goes there too — slightly redundant but kept for
  consistency. On a drill-down (e.g. Battery Health, Wi-Fi Info,
  System → Memory / CPU / Top 10, per-process pages from any of
  those, Tools → Disks → per-disk page), Back goes to the
  immediate parent — process pages return to their list, disk pages
  return to the disks list, etc.

Drill-down panels also surface a `🔄 Refresh` button alongside Back so
you can re-sample the same view without leaving it.

### Refresh

Some dashboards (Battery, System) have a 🔄 Refresh button instead of
auto-polling. macontrol does not push state changes; if you leave a
dashboard open and the underlying state changes, you have to tap
Refresh to see it.

This is intentional — auto-refreshing every dashboard would mean a
constant stream of `editMessageText` calls and would drain the
daemon's CPU even when nobody's looking.

## 3. Flows (multi-step text input)

Some actions need data you can't encode in a button. Set-exact-volume
needs a number from 0–100; join-wifi needs an SSID and a password;
notify needs title and body. Those open a **flow**.

Example: tap **🔊 Sound → Set exact value…**

```text
You:                      [tap "Set exact value…"]
Bot:  Enter target volume (0-100). Reply /cancel to abort.
You:                      42
Bot:  ✅ Volume set — 42% · muted: false
```

The flow takes over the chat for the duration. Properties:

- **Per-chat**: each chat (your DM with the bot) has at most one active
  flow. Starting a new one cancels the old one.
- **Times out**: 5 minutes of inactivity and the flow is dropped
  silently. Sending text after that is treated as a regular non-command
  message and ignored.
- **Cancellable**: send `/cancel` to abort at any time.
- **Multi-step**: some flows have more than one step. Join-Wifi asks
  for the SSID, then the password.

Flow prompts arrive as new messages (not edits to the dashboard you
came from). When the flow finishes, it sends a confirmation message —
the original dashboard remains where you left it.

## 4. Confirm sub-keyboards (destructive actions)

For actions that you can't undo with a tap (`Restart`, `Shutdown`,
`Logout`), the first tap doesn't fire the action. It edits the message
to a confirm sub-keyboard:

```text
⚠ Confirm shutdown

This will affect your Mac immediately. Tap Confirm to proceed.

[ ✅ Confirm ] [ ✖ Cancel ]
```

- **Confirm** runs the action.
- **Cancel** edits the message back to the **parent dashboard**
  (e.g. Cancel from a Power confirm returns to Power, not Home; Cancel
  from a System → Force Kill confirm returns to the per-process page).

There's no time-out on the confirm step — if you don't want to commit,
just don't tap Confirm. The bot doesn't ask twice.

## Callback protocol (for the curious)

Every inline button carries a `callback_data` string in the format
`<ns>:<action>[:<arg>]`. The router parses this on every tap. See
[Reference → Callback protocol](../reference/callback-protocol.md) for
the full namespace list and how long arguments (Bluetooth MACs, SSIDs)
get squeezed through Telegram's 64-byte limit via a TTL shortmap.

## What about the audit trail?

Every command, callback, and flow event is written to the rotating log
at `~/Library/Logs/macontrol/macontrol.log` with the issuing user's
Telegram ID. See [Operations → Logs](../operations/logs.md).

Telegram itself also keeps the chat history visible to you and the bot
forever — you can scroll back to see exactly what was sent and when.
