# Callback protocol

Every inline-keyboard button carries a `callback_data` string that
tells the dispatcher which handler to invoke. The format is fixed,
the namespace list is fixed, and the size limit is enforced at encode
time.

## The format

```text
<namespace>:<action>[:<arg1>[:<arg2>...]]
```

Examples:

```text
snd:open                 # open the Sound dashboard
snd:up:5                 # increase sound by 5
snd:mute                 # mute toggle
pwr:restart              # show restart confirm
pwr:restart:ok           # confirmed restart
bt:conn:abc123def4       # connect to BT device with shortmap id "abc123def4"
nav:home                 # navigate to home grid
```

Encoding rules:

- Three components separated by colons.
- Namespace is 2–3 lowercase characters.
- Action is short lowercase (verb).
- Args are positional, optional. May contain colons themselves (parsed
  by splitting on the first two colons only — any further colons are
  part of the args).

## The 64-byte limit

Telegram caps `callback_data` at **64 bytes**. macontrol enforces this
both ways:

- `callbacks.Encode(ns, action, args...)` panics if the result exceeds
  64 bytes (caller error — should never reach the user).
- `callbacks.Decode(raw)` rejects strings over 64 bytes (defensive
  against malformed input).

For args that might overflow (Bluetooth MACs, SSIDs, process names,
Shortcut names), use the [shortmap](#the-shortmap) instead of inlining
the value.

## Namespaces

Twelve total. Each maps to a `handle<Category>` function in
`internal/telegram/handlers/`.

| Constant | Value | Handler | Categories it serves |
|---|---|---|---|
| `NSSound` | `snd` | `handleSound` | 🔊 Sound |
| `NSDisplay` | `dsp` | `handleDisplay` | 💡 Display |
| `NSPower` | `pwr` | `handlePower` | ⚡ Power |
| `NSWifi` | `wif` | `handleWiFi` | 📶 Wi-Fi |
| `NSBT` | `bt` | `handleBluetooth` | 🔵 Bluetooth |
| `NSBattery` | `bat` | `handleBattery` | 🔋 Battery |
| `NSSystem` | `sys` | `handleSystem` | 🖥 System |
| `NSMedia` | `med` | `handleMedia` | 📸 Media |
| `NSNotify` | `ntf` | `handleNotify` | 🔔 Notify |
| `NSTools` | `tls` | `handleTools` | 🛠 Tools |
| `NSNav` | `nav` | `handleNav` | navigation (home button) |

The full list lives in `internal/telegram/callbacks/data.go` as
constants. Handlers reference these constants, never string literals,
to prevent typos.

## Per-namespace action conventions

Every namespace has at least an `open` action (initial dashboard
render) and a `refresh` action where the dashboard reads state.
Namespaces with category-specific actions add them on top:

| Namespace | Common actions |
|---|---|
| `snd` | `open`, `refresh`, `up:<delta>`, `down:<delta>`, `mute`, `unmute`, `max`, `set` (triggers flow) |
| `dsp` | `open`, `refresh`, `up:<delta>`, `down:<delta>`, `set`, `screensaver` |
| `pwr` | `open`, `lock`, `sleep`, `restart`, `restart:ok`, `shutdown`, `shutdown:ok`, `logout`, `logout:ok`, `keepawake`, `cancelawake` |
| `wif` | `open`, `refresh`, `toggle`, `info`, `dns:cf/google/reset`, `speedtest`, `join` (flow) |
| `bt` | `open`, `refresh`, `toggle`, `paired`, `conn:<short>`, `disc:<short>` |
| `bat` | `open`, `refresh`, `health` |
| `sys` | `open`, `info`, `temp`, `mem`, `cpu`, `top`, `kill` (flow) |
| `med` | `open`, `shot`, `shot:silent`, `record` (flow), `photo` |
| `ntf` | `open`, `send` (flow), `say` (flow) |
| `tls` | `open`, `clip:get`, `clip:set` (flow), `tz` (flow), `synctime`, `disks`, `shortcut` (flow) |
| `nav` | `home` |

Actions ending in `:ok` are confirmation-fires (second-tap on
destructive actions).

## Decoding

```go
data, err := callbacks.Decode("snd:up:5")
// data.Namespace == "snd"
// data.Action    == "up"
// data.Args      == []string{"5"}
```

Errors:

- Empty string → `"empty callback data"`
- Over 64 bytes → `"callback data exceeds 64 bytes"`
- Fewer than 2 components → `"callback data must have at least namespace and action"`

The router catches all errors and toasts `"Bad callback data."` to
the user, then logs the error.

## Encoding

```go
s := callbacks.Encode("snd", "up", "5")
// s == "snd:up:5"
```

If the result exceeds 64 bytes, **`Encode` panics**. This is
intentional — callers should never construct callback data that's too
long. If the args might overflow, use `ShortMap.Put` first and pass
the short id.

```go
// BAD: panics for long SSIDs
s := callbacks.Encode("wif", "join", "Some Long SSID Name")

// GOOD: stash the SSID, embed only the id
id := deps.ShortMap.Put("Some Long SSID Name")
s := callbacks.Encode("wif", "join", id)
```

## The shortmap

`callbacks.ShortMap` is an in-memory `map[string]value` with TTL.
Used for arguments that might exceed the 64-byte budget.

```go
m := callbacks.NewShortMap(15 * time.Minute)

id := m.Put("00-11-22-33-44-55-66")  // returns 10-char base32 id
// callback_data: "bt:conn:" + id  (17 chars total)

value, ok := m.Get(id)
// "00-11-22-33-44-55-66", true
```

Properties:

- **Key format**: 10 base32 chars (50 bits of entropy). Collision
  probability for typical use (< 100 active entries) is ~10⁻¹⁰.
- **TTL**: 15 minutes from put, sliding window not enforced (a Get
  doesn't refresh).
- **Janitor**: runs every TTL/2 (= 7.5 min by default), removes
  expired entries.
- **Persistence**: none. Daemon restart wipes the map.

When a handler tries to `Get` an expired or missing id, it should
toast "session expired" and direct the user back to the listing
keyboard (which re-issues fresh ids).

## Sample dispatch

A user taps **🔊 Sound → +5**:

```text
1. Telegram client → POST callback_query with data="snd:up:5"
2. bot.dispatch → routes to deps.Calls.Handle (CallbackRouter)
3. CallbackRouter.Handle:
   a. callbacks.Decode("snd:up:5") → {Namespace:"snd", Action:"up", Args:["5"]}
   b. switch data.Namespace → handleSound
4. handleSound(ctx, deps, q, data):
   a. switch data.Action → "up" branch
   b. delta := atoi(data.Args[0]) // 5
   c. Reply.Ack(ctx, q) // dismiss spinner
   d. state, err := deps.Services.Sound.Adjust(ctx, delta)
   e. text, kb := keyboards.Sound(state)
   f. Reply.Edit(ctx, q, text, kb)  // editMessageText with new state
5. User sees the message update with "61% · unmuted"
```

## Why this design

See [Architecture → Design decisions](../architecture/design-decisions.md#why-the-64-byte-callback-protocol-with-a-shortmap)
for the rationale: short, machine-routable strings + an out-of-band
store for long values.

The alternative would be embedding routing info in message metadata
(message ID + button position → state lookup), which is what older
bot frameworks do. It's brittle (loses state on `editMessageText`)
and clunky (every state lookup needs a database). callback_data
strings are the modern Telegram idiom.
