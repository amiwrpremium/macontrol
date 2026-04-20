# Testing

What macontrol tests cover, what they don't, and how to run them.

## Coverage at a glance

```text
ok    github.com/amiwrpremium/macontrol/internal/version            100.0%
ok    github.com/amiwrpremium/macontrol/internal/telegram/keyboards 100.0%
ok    github.com/amiwrpremium/macontrol/internal/domain/tools       100.0%
ok    github.com/amiwrpremium/macontrol/internal/domain/system      100.0%
ok    github.com/amiwrpremium/macontrol/internal/domain/power       100.0%
ok    github.com/amiwrpremium/macontrol/internal/domain/notify      100.0%
ok    github.com/amiwrpremium/macontrol/internal/domain/display     100.0%
ok    github.com/amiwrpremium/macontrol/internal/capability         100.0%
ok    github.com/amiwrpremium/macontrol/internal/runner              98.2%
ok    github.com/amiwrpremium/macontrol/internal/domain/bluetooth    96.6%
ok    github.com/amiwrpremium/macontrol/internal/domain/sound        94.6%
ok    github.com/amiwrpremium/macontrol/internal/domain/wifi         92.7%
ok    github.com/amiwrpremium/macontrol/internal/domain/battery      92.5%
ok    github.com/amiwrpremium/macontrol/internal/domain/media        89.7%
ok    github.com/amiwrpremium/macontrol/internal/telegram/handlers   88.5%
ok    github.com/amiwrpremium/macontrol/internal/config              88.1%
ok    github.com/amiwrpremium/macontrol/internal/domain/status       87.5%
ok    github.com/amiwrpremium/macontrol/internal/telegram/flows      85.5%
ok    github.com/amiwrpremium/macontrol/internal/telegram/callbacks  83.3%
ok    github.com/amiwrpremium/macontrol/internal/telegram/bot        83.1%
ok    github.com/amiwrpremium/macontrol/internal/telegram/telegramtest 79.4%
ok    github.com/amiwrpremium/macontrol/cmd/macontrol                 9.1%
```

Total: ~80% across the tree. CI enforces a per-package floor via
`.testcoverage.yml` (currently set as a soft warning, not a blocking
gate — flip to blocking when CI history is stable).

## How to run

### Quick, no race detector

```bash
make test
# or:
go test ./...
```

### With race detector + coverage profile

```bash
make test-race
# or:
go test -race -coverprofile=coverage.out ./...
```

### HTML coverage report

```bash
make cover
# generates coverage.html — open in a browser
```

### Coverage floor enforcement

```bash
make cover-floor
# runs go-test-coverage against .testcoverage.yml
```

## Test infrastructure

### `runner.Fake` — the subprocess mock

Every domain test uses `runner.NewFake()` to register expected commands
and their canned responses. Tests don't shell out to real macOS CLIs.

```go
f := runner.NewFake().
    On("pmset -g batt", " -InternalBattery-0 (id=1)\t78%; charging; 1:00 remaining present: true\n", nil).
    On("system_profiler SPPowerDataType", "Cycle Count: 312\nCondition: Normal\n", nil)

svc := battery.New(f)
st, err := svc.Get(context.Background())
```

`Fake` records every call so tests can assert what was invoked and
with what arguments:

```go
calls := f.Calls()
// calls[0] = {Sudo: false, Name: "pmset", Args: ["-g", "batt"]}
```

The `Sudo` flag distinguishes `f.Exec(...)` from `f.Sudo(...)` — the
production path prepends `sudo -n`, but the Fake records the original
args (no `-n` injection) so test fixtures stay readable.

### `telegramtest.NewBot` — the Telegram API stub

Handler and command tests don't call real Telegram. Instead:

```go
b, rec := telegramtest.NewBot(t)

// b is a real *tgbot.Bot pointed at an in-process httptest.Server
// rec captures every outgoing API call

deps := &bot.Deps{Bot: b, ...}
err := handler.Handle(ctx, deps, update)

calls := rec.ByMethod("editMessageText")
// assert on calls[0].Fields["chat_id"], calls[0].Fields["text"], etc.
```

The harness:

- Uses `tgbot.WithServerURL(srv.URL)` to point the real Bot client at
  the test server.
- Uses `tgbot.WithSkipGetMe()` to avoid the startup `getMe` call.
- Always responds with `{"ok":true,"result":{...}}` so the library
  doesn't error out.
- Records every request with the parsed multipart fields.

This means tests exercise the **real Bot client code paths** — URL
construction, multipart payload building, JSON unmarshalling — not
just our wrapper.

### `handlers.newHarness` — the integrated test harness

For handler tests, `internal/telegram/handlers/helpers_test.go`
provides `newHarness(t)` which wires:

- A real `*tgbot.Bot` via `telegramtest.NewBot`
- A `runner.Fake` shared by all domain services
- A real `flows.Registry` and `callbacks.ShortMap`
- All 11 domain services constructed against the fake runner
- A real `bot.Deps` with everything plugged in

Tests then:

```go
h := newHarness(t)
h.Fake.On("osascript -e ...", "60,false", nil)

err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
    newCallbackUpdate("id", "snd:open"))

last := h.Recorder.Last()
// assert last.Method, last.Fields["chat_id"], last.Fields["text"]
```

The harness is the highest-fidelity path short of a real Telegram +
real Mac integration test.

## What's covered

| Area | Coverage |
|---|---|
| Domain logic (parser, business rules, error paths) | High (≥90% per package) |
| Keyboard layout (button text, callback data, version gates) | 100% |
| Callback routing (namespace dispatch, error fallback) | High |
| Flow state machines (multi-step, validation, cancel) | High |
| Bot dispatch (whitelist, command/callback/flow routing, panic recovery) | 83% |
| Config loader (env + file precedence, friendly errors) | 88% |
| Runner (exec, error capture, timeout, fake) | 98% |
| Capability detection (version parsing, feature gates) | 100% |

## What's NOT covered (and why)

| Area | Why not |
|---|---|
| `cmd/macontrol/daemon.go` `runDaemon()` | Wires signal handlers, calls `bot.Start` (blocking long-poll). Practically integration-only. |
| `cmd/macontrol/setup.go` interactive prompts | `term.ReadPassword` reads from a real TTY. Refactoring to inject a `Reader` would help; not done yet. |
| `cmd/macontrol/service.go` launchctl calls | Shells out to `launchctl bootstrap/bootout` which require launchd; not testable on Linux CI. |
| Real macOS subprocess invocations (any `osascript`, `pmset`, etc.) | Domain tests use `runner.Fake`, not real binaries. The mapping from "callsite ↔ exact CLI invocation" is verified by reading code, not by running it. |
| TCC-prompted operations (screencapture, imagesnap) | Triggers macOS UI prompts. Not automatable in CI. |

The pragmatic bar: **everything that can be tested in a Linux CI
runner, is**. Everything else is verified by manual smoke-test on a
real Mac before each release.

## Test layout convention

| Test file | Tests what |
|---|---|
| `<pkg>/<pkg>_test.go` | The package's exported API, table-driven where possible |
| `<pkg>/<pkg>_internal_test.go` | Unexported helpers (uses `package <pkg>` not `<pkg>_test`) |
| `<pkg>/<feature>_test.go` | A specific feature when the main file would balloon (`runner/extras_test.go`, `bot/dispatch_test.go`) |
| `handlers/helpers_test.go` | Shared harness construction |

## Race detector

CI runs `go test -race` on every push. The race detector catches:

- The `flows.Registry` and `callbacks.ShortMap` mutex usage (good
  coverage from concurrent test code paths).
- The `runner.Fake.Calls()` snapshot semantics.
- Any shared state in handler dispatch.

If you add new shared state, put it behind a mutex and add a
race-detector test (call from N goroutines, assert no races).

## Coverage philosophy

We test **behavior**, not implementation. A test that asserts "calling
`Set(75)` invokes `osascript -e 'set volume output volume 75'`" is
useful (regression bait); one that asserts internal struct field
values is brittle.

For UI code (keyboards, callbacks):

- Round-trip tests: every button's `callback_data` parses cleanly via
  `callbacks.Decode`, every callback name routes to a handler.
- State-dependent variation tests: muted vs unmuted shows different
  buttons; off-vs-on Wi-Fi shows different toggle text.
- Version gate tests: features hidden when feature flag is false.

For flows:

- Each step's prompt text is asserted.
- Invalid input re-prompts (Done=false).
- Valid input ends the flow (Done=true) and produces the right
  follow-up action.

## Adding tests for new code

When you add a new feature:

1. **Domain method** → add a `_test.go` table-driven test using
   `runner.NewFake()`. Cover happy path, error from runner, and any
   parsing branches.
2. **Keyboard** → add a layout assertion in the relevant
   `keyboards_test.go` (button presence, callback round-trip,
   version-gate hidden).
3. **Handler** → add a test in `handlers/<ns>_test.go` (or
   `remaining_test.go` for new actions on existing categories) using
   the `newHarness` pattern.
4. **Flow** → add a test in `flows/flows_test.go` covering Start,
   invalid input, valid input.
5. **CLI subcommand** → at minimum, test parseable helpers (anything
   not behind a TTY or launchctl).

See [Development → Adding a capability](../development/adding-a-capability.md)
for the full file checklist.

## When tests are wrong

If a test passes but the feature doesn't work in real life, the test
isn't testing what you think. Common patterns:

- **Fake registered with the wrong cmdline**: `f.On("brightness 0.5", ...)`
  but the code actually invokes `brightness 0.500` (3-decimal format).
  Result: the Fake's prefix-matching falls back to "no rule" and the
  test silently uses an empty response.
- **Asserting on send when the code uses edit**: tests that assert
  `rec.ByMethod("sendMessage")` but the handler actually edits via
  `editMessageText`.
- **Update.From vs Update.CallbackQuery.From**: forgetting to set the
  right field on the test Update means the whitelist check rejects
  the test's user ID.

The harness's `Recorder.Calls()` dump is your best friend when
debugging — print it after the failing assertion to see what the
handler actually did.
