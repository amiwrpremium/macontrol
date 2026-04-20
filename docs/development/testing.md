# Testing

How to run tests, what each helper does, and what coverage looks like.

For the full coverage matrix and design rationale, see
[Architecture → Testing](../architecture/testing.md).

## Quick reference

```bash
make test          # go test ./...
make test-race     # go test -race -coverprofile=coverage.out ./...
make cover         # generate coverage.html
make cover-floor   # enforce per-package thresholds
```

CI runs `test-race` on every push (ubuntu + macos-14 matrix).

## Test infrastructure

### `runner.Fake` — subprocess mock

Every domain test uses this. Register expected commands and their
canned responses; the Fake records every call for assertions.

```go
f := runner.NewFake().
    On("pmset -g batt", "78%; charging; 1:00 remaining\n", nil).
    On("system_profiler SPPowerDataType", "Cycle Count: 312\n…", nil)

svc := battery.New(f)
status, err := svc.Get(context.Background())

// Assertions on what got called:
calls := f.Calls()
// calls[0] = {Sudo: false, Name: "pmset", Args: ["-g", "batt"]}
```

The `On` rule key is the full command line as `name + " " + args`. The
Fake's dispatch tries exact match first, then falls back to prefix
match (so `On("screencapture ", ...)` matches any screencapture
invocation).

### `telegramtest.NewBot` — fake Telegram API

Used by handler and command tests. Spins up an httptest.Server, points
a real `*tgbot.Bot` at it via `WithServerURL`, records every
outgoing API call.

```go
b, rec := telegramtest.NewBot(t)

// b is a real *tgbot.Bot wired to an in-process server
// rec captures every outgoing call

// Use the bot:
b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: 1, Text: "hi"})

// Assert:
calls := rec.Calls()
// calls[0] = {Method: "sendMessage", Fields: {"chat_id": "1", "text": "hi"}, Files: {}}
```

The Recorder also has `.ByMethod("sendMessage")`, `.Last()`, `.Reset()`.

### `handlers.newHarness` — full integration harness

For handler tests. Wires:

- A real `*tgbot.Bot` via `telegramtest.NewBot`
- A `runner.Fake` shared by all domain services
- All 11 domain services constructed against the fake runner
- `flows.Registry`, `callbacks.ShortMap`
- A real `bot.Deps` with everything plugged in

```go
h := newHarness(t)
h.Fake.On("osascript -e ...", "60,false", nil)

err := handlers.NewCallbackRouter().Handle(ctx, h.Deps,
    newCallbackUpdate("id", "snd:open"))

last := h.Recorder.Last()
// assertions on last.Method, last.Fields["text"], etc.
```

This is the highest-fidelity test path short of a real Telegram +
real Mac integration.

## Patterns by layer

### Domain (pure logic)

```go
func TestGet(t *testing.T) {
    t.Parallel()
    f := runner.NewFake().On("expected cmd", "stdout", nil)
    got, err := pkg.New(f).Method(context.Background())
    if err != nil { t.Fatal(err) }
    if got != "expected" { t.Errorf("got %q", got) }
}
```

Always:

- `t.Parallel()` unless the test mutates env vars or shared state.
- Cover happy path + each error branch + each parsing edge case.
- For multi-call methods (e.g. Wi-Fi Get → Interface + Power + SSID),
  register every expected call.

### Keyboards (pure UI)

```go
func TestSound_Unmuted(t *testing.T) {
    t.Parallel()
    text, kb := keyboards.Sound(sound.State{Level: 60, Muted: false})
    if !strings.Contains(text, "60%") { t.Errorf("text = %q", text) }
    assertContainsButton(t, kb, "🔇 Mute")
    assertAllRoundtrip(t, kb)  // every callback_data round-trips through Decode
}
```

Helpers in `keyboards_test.go`:

- `allCallbackData(kb)` — extract every button's data.
- `assertAllRoundtrip(t, kb)` — every data string parses cleanly.
- `assertContainsButton(t, kb, substr)` — at least one button has
  that text.
- `assertNavPresent(t, kb)` — the standard 🏠 Home row exists.

### Flows

```go
func TestSetVolumeFlow_Valid(t *testing.T) {
    t.Parallel()
    f := /* runner.NewFake with the right rules */
    flow := flows.NewSetVolume(sound.New(f))

    resp := flow.Handle(context.Background(), "42")
    if !resp.Done { t.Fatal("expected Done") }
    if !strings.Contains(resp.Text, "42") { t.Errorf("text = %q", resp.Text) }
}

func TestSetVolumeFlow_Invalid(t *testing.T) {
    t.Parallel()
    flow := flows.NewSetVolume(sound.New(runner.NewFake()))
    for _, input := range []string{"abc", "-1", "101"} {
        resp := flow.Handle(context.Background(), input)
        if resp.Done {
            t.Errorf("input %q should not terminate", input)
        }
    }
}
```

For multi-step flows (joinwifi), test step-by-step:

```go
func TestJoinWifiFlow_TwoStep(t *testing.T) {
    flow := flows.NewJoinWifi(...)
    first := flow.Handle(ctx, "MyNetwork")
    if first.Done { t.Fatal("first step should not be Done") }
    second := flow.Handle(ctx, "password")
    if !second.Done { t.Fatal("second step should be Done") }
}
```

### Handlers

```go
func TestSnd_Up(t *testing.T) {
    t.Parallel()
    h := newHarness(t)
    h.Fake.
        On("osascript -e set v to output volume…", "60,false", nil).
        On("osascript -e set volume output volume 65", "", nil)

    if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
        newCallbackUpdate("id", "snd:up:5")); err != nil {
        t.Fatal(err)
    }
    if len(h.Recorder.ByMethod("editMessageText")) != 1 {
        t.Fatal("expected editMessageText")
    }
}
```

`newCallbackUpdate(id, data)` and `newMessageUpdate(text)` build
typed `*models.Update` values with the whitelisted user ID baked in.

## Coverage targets

| Package | Target |
|---|---|
| Domain layers | ≥ 90% |
| Telegram UI layers | ≥ 80% |
| Core (runner, capability, config, version) | ≥ 90% |
| `cmd/macontrol` | ≥ 5% (orchestration is hard to unit-test) |

`.testcoverage.yml` enforces these as a soft check (currently
non-blocking).

## Race detector

CI runs `-race` on every push. The race detector catches:

- Concurrent reads/writes to `flows.Registry` and `callbacks.ShortMap`
  (both have mutexes; tests exercise concurrent access).
- Shared state in handler dispatch (rare, mostly avoided).
- The `runner.Fake` `Calls()` snapshot semantics.

If you add new shared state, add a race-detector test (call from N
goroutines, assert no panics).

## Adding a test for new code

1. **Domain method**: add a table-driven test in
   `internal/domain/<pkg>/<pkg>_test.go`. Cover happy path, error
   from runner, every parsing branch.

2. **Keyboard**: add a layout assertion in `keyboards_test.go`.
   Button presence, callback round-trip, version-gate hidden.

3. **Flow**: add to `flows/flows_test.go` covering Start, invalid
   input (Done=false), valid input (Done=true).

4. **Handler**: add to `handlers/<ns>_test.go` (or
   `remaining_test.go` for new actions on existing categories) using
   the `newHarness` pattern.

5. **CLI subcommand**: at minimum, test pure helpers (anything not
   behind a TTY or `launchctl`).

## When tests are wrong

If a test passes but the feature doesn't work in real life, the test
isn't testing what you think. Common culprits:

- **Fake registered with the wrong cmdline** — e.g.
  `f.On("brightness 0.5", ...)` but the code actually invokes
  `brightness 0.500` (3-decimal format). The Fake's prefix-matching
  falls back to "no rule" and the test silently uses an empty
  response.
- **Asserting on send when the code uses edit** — tests that assert
  `rec.ByMethod("sendMessage")` but the handler actually edits via
  `editMessageText`.
- **Update.From vs Update.CallbackQuery.From** — forgetting to set
  the right field on the test Update means the whitelist check
  rejects the test's user ID.

The harness's `Recorder.Calls()` is your friend when debugging — print
it after the failing assertion to see what the handler actually did:

```go
t.Logf("recorder calls: %+v", h.Recorder.Calls())
```

## Full coverage HTML

```bash
make cover
open coverage.html
```

Red lines are uncovered. Most red lines fall into:

- `os.Exit` paths — by design, not unit-testable.
- Signal handling in `cmd/macontrol/daemon.go` — integration only.
- Defensive panic-recovery branches.

## Running tests in editor

For VS Code with the Go extension: it picks up tests automatically.
Click "run test" / "debug test" above each function.

For neovim with `vim-go` or `nvim-dap-go`: same.

For terminal: `go test -run TestName ./internal/domain/sound/...`.
