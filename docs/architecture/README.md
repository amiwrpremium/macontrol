# Architecture

How the daemon is wired internally. Read these in order if you want
the big picture; jump directly to the relevant doc if you're chasing
a specific concern.

## What's here

- **[Overview](overview.md)** — runtime topology, dispatch loop, ASCII
  data-flow diagram showing Telegram update → router → handler →
  domain → runner → reply.
- **[Project layout](project-layout.md)** — file-by-file walk of
  `cmd/` and `internal/` with what lives where and why.
- **[Design decisions](design-decisions.md)** — the WHY behind every
  non-obvious choice. Read this if "why don't you just X" comes to
  mind.
- **[Testing](testing.md)** — coverage matrix, test infrastructure
  (`runner.Fake`, `telegramtest`), what's covered and what's not.

## At a glance

```text
   Telegram (long-poll)
         │
         ▼
   bot.dispatch  ──── reject if sender not in whitelist
         │
         ├── Update.Message + isCommand → commands.Router
         ├── Update.Message (other text) → flows.Registry.Active(chat) → Flow.Handle
         └── Update.CallbackQuery → callbacks.Router
                                          │
                                          ├── parses ns:action[:arg]
                                          ▼
                                    handlers.<category>
                                          │
                                          ▼
                                    domain.<category>.Service
                                          │
                                          ▼
                                    runner.Exec / Sudo  ───▶  os/exec → macOS CLI
                                          │
                                          ▼
                                       state
                                          │
                                          ▼
                                  Reply.Edit (in place)
                                  Reply.Send (new message)
                                  Reply.SendPhoto / SendVideo
```

## Three-layer separation

| Layer | Lives in | Knows about | Doesn't know about |
|---|---|---|---|
| **Domain** | `internal/domain/*` | macOS CLIs, the runner | Telegram, keyboards, callbacks |
| **Telegram UI** | `internal/telegram/*` | Domain services, Telegram API | macOS CLIs (only via domain) |
| **Entry point** | `cmd/macontrol/*` | Both, plus config and signals | nothing else |

The strict layering is enforced by code review (CI doesn't currently
fail on cross-layer imports, but the linter could be extended). The
benefit:

- Domain code is **testable without Telegram** — `runner.Fake`
  replaces the subprocess shim; tests run on Linux fine.
- Telegram code is **testable without macOS** — `telegramtest`
  provides an httptest-backed Bot; domain services use `runner.Fake`.
- Each layer has a single concern and is small enough to fit in your
  head.

## Process model

A single Go process. No goroutine pool, no worker model:

- Long-poll loop runs in `bot.Start`'s goroutine.
- Every Telegram update is dispatched in a fresh goroutine (the
  go-telegram/bot library spawns one per update by default).
- Each dispatch either calls a router synchronously, or installs a
  flow and returns immediately.
- Janitor goroutines run in the background:
  - `flows.Registry` evicts inactive flows every 2.5 minutes.
  - `callbacks.ShortMap` evicts expired short-ids every 7.5 minutes.

No long-running goroutines per chat. No background polling beyond
Telegram's own long-poll.
