# Architecture overview

The big picture: what runs, where, and how a Telegram tap becomes a
macOS subprocess invocation.

## Runtime topology

```text
┌──────────────────────────┐                ┌──────────────────────────────┐
│  Telegram Bot API server │                │  macontrol daemon (the Mac)  │
│                          │                │                              │
│  api.telegram.org        │ ◀── HTTPS ──▶ │  single Go binary            │
│                          │   long-poll    │  darwin/arm64                │
│                          │                │                              │
└──────────────────────────┘                │  - LaunchAgent supervises    │
                                            │  - whitelist on user id      │
                                            │  - subprocess → macOS CLIs   │
                                            └──────────────────────────────┘
```

The daemon runs **on the Mac it controls**. It speaks to Telegram via
outbound HTTPS long-polling — no inbound port, no public IP needed.
This is the same pattern most "control my home server from Telegram"
bots use, and it works behind any NAT.

## The dispatch loop

`internal/telegram/bot/bot.go` wires everything together.

```text
                                   ┌───────────────────────────────────────────────────┐
                                   │              bot.Start(ctx, token, deps)          │
                                   │                                                   │
   go-telegram/bot library  ──────▶│  long-poll loop running in its own goroutine      │
                                   │                                                   │
                                   │      Update lands ──▶ bot.dispatch                │
                                   │                            │                      │
                                   │                            ▼                      │
                                   │             ┌─ recover middleware ───────┐        │
                                   │             │ catches and logs panics    │        │
                                   │             └────────────┬───────────────┘        │
                                   │                          │                        │
                                   │             ┌─ whitelist check ────────────┐      │
                                   │             │ deps.Whitelist.Allows(update)│      │
                                   │             │   - reject + log if false    │      │
                                   │             └────────────┬─────────────────┘      │
                                   │                          │                        │
                                   │              ┌───────────┴────────────┐           │
                                   │              ▼                        ▼           │
                                   │     update.CallbackQuery       update.Message     │
                                   │              │                        │           │
                                   │              ▼                        ▼           │
                                   │    deps.Calls.Handle()      isCommand?            │
                                   │              │                  yes/no            │
                                   │              ▼                ▼      ▼            │
                                   │     handlers.CallbackRouter   /   plain text      │
                                   │              │              cmd        │          │
                                   │              ▼              │          ▼          │
                                   │     parse <ns>:<act>:<arg>  ▼  flows.Active(chat) │
                                   │              │     deps.Commands.Handle  │        │
                                   │              ▼                           ▼        │
                                   │     dispatch by namespace          Flow.Handle    │
                                   │     to handle<Category>            ┌─────┴──┐    │
                                   │              │                     │        │    │
                                   │              ▼                     ▼        │    │
                                   │    domain.<Category>.Service   Response{    │    │
                                   │              │                  Text,        │    │
                                   │              ▼                  Markup,      │    │
                                   │     runner.Exec / Sudo          Done bool}   │    │
                                   │              │                  │            │    │
                                   │              ▼                  ▼            │    │
                                   │       os/exec subprocess     bot.SendMessage │    │
                                   │              │                  │            │    │
                                   │              ▼                  ▼            │    │
                                   │     Reply.Edit / Send         user sees text │    │
                                   │              │                  │            │    │
                                   │              ▼                  └────────────┘    │
                                   │       user sees update                            │
                                   └───────────────────────────────────────────────────┘
```

Every Telegram update follows one of three paths:

### Path 1 — Callback query (inline button tap)

```text
update.CallbackQuery
  → deps.Calls.Handle(ctx, deps, update)
  → handlers.CallbackRouter.Handle
    → callbacks.Decode(q.Data) → {Namespace, Action, Args}
    → switch Namespace → handle<Category>(ctx, deps, q, data)
      → deps.Services.<Category>.SomeMethod(ctx, args)
        → runner.Exec(ctx, "binary", args...)
          → os/exec.CommandContext → macOS CLI
        ← stdout, error
      ← typed state
      → Reply.Edit(ctx, q, newText, newKeyboard)
        → bot.EditMessageText
```

The user sees the message they tapped a button on update with new state
and the same keyboard. No new message is sent unless the action
explicitly does it (screenshots, record, photos).

### Path 2 — Slash command (`/menu`, `/status`, etc.)

```text
update.Message.Text starts with "/"
  → deps.Commands.Handle(ctx, deps, update)
  → handlers.CommandRouter.Handle
    → parseCommand(text) → cmd, rest
    → switch cmd → cmd<Name>(ctx, deps, update)
      → may call deps.Services.<Category>.SomeMethod
        → runner.Exec
      → bot.SendMessage(ctx, ...)
```

Slash commands always send a NEW message (no message to edit yet).

### Path 3 — Plain text (consumed by an active flow)

```text
update.Message.Text not "/" and not empty
  → bot.dispatchFlow
  → flow, ok = deps.Flows.Active(chatID)
  → if ok:
      response = flow.Handle(ctx, text)
      if response.Done: deps.Flows.Finish(chatID)
      bot.SendMessage(ctx, chatID, response.Text, response.Markup)
  → if not ok:
      log.Debug("ignored plain text")
```

Flows handle multi-step interactions (set-exact-volume, join-wifi,
notify-send, etc.). See [Usage → UX model](../usage/ux-model.md) for
the user-facing description.

## Where state lives

macontrol is mostly stateless. There are two in-memory caches:

### `flows.Registry` — chat → active flow

```text
map[chatID]*entry{
  flow:      Flow,            // Name(), Start(), Handle()
  lastTouch: time.Time,       // for TTL eviction
}
```

- One entry per chat.
- TTL: 5 minutes since last touch.
- Janitor goroutine sweeps every 2.5 minutes.
- Survives nothing — daemon restart wipes all in-flight flows. The
  user has to start over from the keyboard.

### `callbacks.ShortMap` — short-id → string

```text
map[id]shortItem{
  value:    string,           // BT MAC, SSID, process name, etc.
  expires:  time.Time,
}
```

- Used when an inline button needs an arg too long to fit in
  Telegram's 64-byte `callback_data` limit.
- TTL: 15 minutes.
- Janitor sweeps every 7.5 minutes.
- Daemon restart wipes the map; existing keyboards become "session
  expired" toasts.

That's all. Everything else (volume, brightness, battery state, etc.)
is read fresh from macOS on every action.

## Configuration loading

`internal/config/config.go`:

1. Check for `MACONTROL_CONFIG` env var. If set, that path is the
   config file.
2. Otherwise, fall back to
   `~/Library/Application Support/macontrol/config.env`.
3. If file exists, load it via `godotenv.Overload` (overlays values
   onto process env).
4. Parse process env into typed `Config` struct via `caarlos0/env`.
5. Validate required fields. Missing → friendly error pointing at
   `macontrol setup`.

This happens once at startup. No hot-reload — `brew services restart`
to pick up changes.

## Logging

`log/slog` + `lumberjack`. Default text handler (key=value), level
configurable via env, output to a rotating file plus stderr in
foreground mode. See [Operations → Logs](../operations/logs.md) for
the format and rotation policy.

## Error model

Three layers, three error styles:

| Layer | Style |
|---|---|
| Domain | Returns typed `error` from every method. Specific errors are wrapped with context (`fmt.Errorf("set volume: %w", err)`). |
| Telegram handlers | Convert errors to user-visible messages via `errEdit(ctx, r, q, header, err)`. The user sees `🔊 Sound — adjust failed\n\n⚠ exit status 1`. |
| Bot dispatcher | Logs unrecovered errors at WARN; recovers panics in middleware and logs at ERROR. Never bubbles up to the long-poll loop. |

A panic deep in a handler does not kill the daemon — the recover
middleware catches it, logs, and the user gets no reply (the equivalent
of a silent drop). The next request works normally.

## Where to read next

- [Project layout](project-layout.md) — what every file does.
- [Design decisions](design-decisions.md) — the why behind the choices
  above.
- [Testing](testing.md) — how each layer is verified.
