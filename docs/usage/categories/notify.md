# 🔔 Notify

Push a desktop notification to your Mac, or speak text through TTS.
Works without brew deps (osascript fallback); richer when
`terminal-notifier` is installed.

## Dashboard

```text
🔔 Notify

Push a desktop notification to your Mac, or speak text through TTS.

[ ✉ Send notification… ]
[ 🗣 Say…                ]
[        🏠 Home          ]
```

## Flows

### ✉ Send notification…

```text
Bot: Send the notification as `title | body`, or just a body.
     /cancel to abort.
You: Build finished | All tests pass — safe to merge.
Bot: ✅ Notified via terminal-notifier.
```

Accepts three input shapes:

| Input | Title | Body |
|---|---|---|
| `title \| body` | "title" | "body" |
| `body` (no pipe) | (none) | "body" |
| `\| body` | (empty) | "body" |

Empty input re-prompts without cancelling.

On macOS, the notification appears in the top-right corner (or the
Notification Center) with the `macontrol` label in the "grouped
by" section.

### 🗣 Say…

```text
Bot: Send the text to speak. /cancel to abort.
You: It's 10 AM.
Bot: ✅ Spoken.
```

Runs `say "<text>"` which uses macOS's built-in TTS voice. Use System
Settings → Accessibility → Spoken Content to configure voice, rate,
and language.

Empty/whitespace-only input re-prompts.

## What's backing this

macontrol tries two transports in order:

1. **`terminal-notifier`** (preferred) — brew formula. Richer
   notifications with `-sound`, `-group`, `-open URL`, `-execute`.
   Supports icons if you pass them.
2. **`osascript`** (fallback) — always available, no brew dep. Simpler
   notifications; no custom sounds or action buttons.

The bot's reply tells you which transport was used:

```text
✅ Notified via terminal-notifier.   # brew formula present
✅ Notified via osascript.           # fallback
```

| Action | Command |
|---|---|
| Rich notify | `terminal-notifier -group macontrol -title T -message B [-sound default]` |
| Basic notify | `osascript -e 'display notification "B" with title "T"'` |
| Say | `say <text>` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#notify).

## Edge cases

### Notifications don't appear

Common causes:

- **Focus mode / Do Not Disturb** is on — the notification is still
  created but macOS silences it. Check System Settings → Focus.
- `macontrol` doesn't have Notification permission — in recent macOS
  versions, apps that send notifications via `UserNotifications`
  framework need authorization. `terminal-notifier` requests this on
  first use but since v2.0 it sometimes silently fails.

**Fix**: System Settings → Notifications → find `terminal-notifier`
(or, if `osascript` is doing the sending, find `Script Editor`), and
ensure notifications are allowed.

### Title-only notifications

Telegram's `|` splitter means a message with only a title and no body
becomes `title | ""` — which macOS shows as the title text with an
empty body below. If you want just the title, send it without a pipe
— macontrol treats it as body and shows it that way.

### `say` voice is wrong

The `say` command uses the system's default voice, which you configure
in Settings → Accessibility → Spoken Content → System Voice. macontrol
doesn't override it.

### Long TTS text

`say` handles multi-paragraph text fine, but long text with unusual
punctuation can mispronounce. There's no post-processing; what you
send is what gets spoken.

## Version gates

None — both notification paths and `say` work on macOS 11+.
