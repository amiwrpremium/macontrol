# 🔊 Sound

Control the Mac's output volume and mute state, plus text-to-speech.
All of this is driven by `osascript`, so no brew dependencies.

## Dashboard

```text
🔊 Sound — 60% · unmuted

[ −5 ] [ −1 ] [ 🔇 Mute ] [ +1 ] [ +5 ]
[          Set exact value…           ]
[            🔊 MAX (100)             ]
[   🔄 Refresh   ]
[          🏠 Home                    ]
```

Header reflects the most recent read from `osascript -e "output volume
of (get volume settings)"`. The volume and muted flag are re-read after
every action that changes them, so the header is always fresh.

## Buttons

### ±5, ±1

Relative adjust. Internally calls `Adjust(delta)`, which clamps to
[0, 100]. Going below 0 sets to 0; above 100 sets to 100 — no wrap.

Each tap triggers one `osascript` round-trip (~50–100 ms depending on
load).

### Mute / Unmute

Toggles macOS's system mute. The button label flips: if currently
unmuted it shows **🔇 Mute**; if muted it shows **🔈 Unmute**.

Muting doesn't change the volume level. Unmuting restores whatever
level was set before muting.

### Set exact value… (flow)

Starts a flow:

```text
Bot: Enter target volume (0-100). Reply /cancel to abort.
You: 75
Bot: ✅ Volume set — 75% · muted: false
```

Accepts any integer 0–100 (inclusive). Non-integers and out-of-range
values re-prompt without cancelling the flow. `/cancel` aborts.

### MAX (100)

Shortcut for Set-to-100. One tap, no flow.

### Refresh

Re-reads state and re-renders the header without changing anything.
Useful if you muted/unmuted from the Mac's menu bar and want the
dashboard to catch up.

### 🏠 Home

Edits the message to the inline home grid.

## What's backing this

Every action ultimately runs an `osascript` command. Examples:

- Read: `osascript -e 'output volume of (get volume settings)'`
- Read muted: `osascript -e 'output muted of (get volume settings)'`
- Set level: `osascript -e "set volume output volume 75"`
- Mute: `osascript -e "set volume output muted true"`

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#sound).

## Edge cases

### AirPods connected / disconnected

macontrol reads the volume as it applies to **the currently active
output device**. If you disconnect AirPods while a dashboard is open,
the volume shown will jump to the built-in speaker's level on the next
Refresh.

### Input (microphone) volume

The dashboard controls **output** only. There's no UI for input volume
because it doesn't affect anything user-visible on most setups. The
underlying code does have `set volume input volume N` available — open
an issue if you want a UI surface for it.

### Text-to-speech

The Sound category doesn't include the `say` action — it lives under
[🔔 Notify](notify.md) because it's more "notification-shaped" than
"audio-shaped" and often used together with notifications.

## Version gates

None. Every feature in this category works on macOS 11+ without any
brew deps.
