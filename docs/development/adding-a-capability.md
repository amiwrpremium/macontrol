# Adding a capability

The 6-file recipe for adding a new feature. Worked example:
`/weather` — a button under 🛠 Tools that fetches the current weather
for the Mac's location.

This is the same pattern every existing capability follows. Once
you've done it once, you can move quickly.

## The 6 files

| # | File | Purpose |
|---|---|---|
| 1 | `internal/domain/weather/weather.go` | Pure macOS-side function: get the data |
| 2 | `internal/domain/weather/weather_test.go` | Domain tests using `runner.Fake` |
| 3 | `internal/telegram/keyboards/tls.go` (edit) | Add a new button to the Tools dashboard |
| 4 | `internal/telegram/handlers/tls.go` (edit) | Handle the new callback action |
| 5 | `internal/telegram/handlers/remaining_test.go` (edit) | Test the handler |
| 6 | `cmd/macontrol/daemon.go` (edit) + `internal/telegram/bot/bot.go` (edit) | Wire the new service into Deps |

Plus docs:

| File | What |
|---|---|
| `docs/usage/categories/tools.md` | Document the new button |
| `docs/reference/macos-cli-mapping.md` | Add the backing command |

## Step 0 — Decide the shape

Before writing code:

- **Is it a button or a flow?** Buttons fire actions immediately
  (read weather, return result). Flows ask for input first (e.g.
  weather for a specific city would need a flow asking for the city).
- **Does it need a brew dep?** Weather via a CLI like `wttr.in` curl
  vs. `weather-cli` brew formula.
- **Does it need TCC permissions?** Weather doesn't; webcam does.
- **Does it need sudo?** Weather doesn't; powermetrics does.

For our example: weather is a button, uses `curl wttr.in` (which
ships in macOS), no TCC, no sudo.

## Step 1 — Domain package

Create `internal/domain/weather/weather.go`:

```go
// Package weather fetches a brief weather summary for the Mac's
// approximate location via wttr.in.
package weather

import (
	"context"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service fetches weather data.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Get returns a one-line weather summary.
func (s *Service) Get(ctx context.Context) (string, error) {
	out, err := s.r.Exec(ctx, "curl", "-s", "wttr.in/?format=3")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

Pattern notes:

- Constructor takes `runner.Runner`, stores it. Always.
- Exposed methods take `context.Context` first.
- Return typed values + error. No `interface{}`.
- Comments on every exported identifier.

## Step 2 — Domain test

Create `internal/domain/weather/weather_test.go`:

```go
package weather_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/weather"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestGet(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("curl -s wttr.in/?format=3", "Istanbul: ☀️ +15°C\n", nil)
	got, err := weather.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "Istanbul: ☀️ +15°C" {
		t.Errorf("got %q", got)
	}
}

func TestGet_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("curl -s wttr.in/?format=3", "", errors.New("no network"))
	if _, err := weather.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
```

Pattern notes:

- Tests live in `package <name>_test` (external) for happy paths.
- `runner.NewFake().On(<full cmdline>, <stdout>, <error>)`.
- `t.Parallel()` for everything that doesn't share state.

Run:

```bash
go test ./internal/domain/weather/...
```

Should pass with 100% coverage on the two test functions.

## Step 3 — Wire into Deps

The bot's `Deps.Services` struct needs a new field for the service.
Edit `internal/telegram/bot/bot.go`:

```go
import (
	// existing imports
	"github.com/amiwrpremium/macontrol/internal/domain/weather"
)

type Services struct {
	// existing fields
	Weather *weather.Service
}
```

Then edit `cmd/macontrol/daemon.go` to construct it:

```go
import (
	// existing imports
	"github.com/amiwrpremium/macontrol/internal/domain/weather"
)

services := bot.Services{
	// existing fields
	Weather: weather.New(r),
}
```

## Step 4 — Add a button to the keyboard

Edit `internal/telegram/keyboards/tls.go` to add the button:

```go
func Tools(features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	text = "🛠 *Tools*"
	rows := [][]models.InlineKeyboardButton{
		// existing rows
		{
			{Text: "🌤 Weather", CallbackData: callbacks.Encode(callbacks.NSTools, "weather")},
		},
	}
	// existing append for shortcut + nav
	return
}
```

The new callback string is `tls:weather`. We're piggy-backing on
the existing `tls` namespace since the action conceptually belongs
to Tools.

## Step 5 — Handle the callback

Edit `internal/telegram/handlers/tls.go` to add a case:

```go
case "weather":
	r.Ack(ctx, q)
	text, err := d.Services.Weather.Get(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🌤 *Weather* — unavailable", err)
	}
	_, kb := keyboards.Tools(d.Capability.Features)
	return r.Edit(ctx, q, "🌤 *Weather*\n"+Code(text), kb)
```

Pattern notes:

- `r.Ack(ctx, q)` — dismiss the spinner immediately.
- Call the service.
- On error: `errEdit` with a header.
- On success: edit the message with the result + the same keyboard
  (so the user can keep tapping).

## Step 6 — Test the handler

Edit `internal/telegram/handlers/remaining_test.go`:

```go
func TestTls_Weather(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("curl -s wttr.in/?format=3", "Istanbul: ☀️ +15°C\n", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:weather")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "Istanbul") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}
```

Run:

```bash
go test ./internal/telegram/handlers/... -run TestTls_Weather
```

## Step 7 — Document it

Two doc updates:

### `docs/usage/categories/tools.md`

Add a section under the existing buttons:

```markdown
### 🌤 Weather

One-tap weather summary for the Mac's approximate location (geo-IP).

Backing: `curl -s wttr.in/?format=3`. No brew dep, no sudo, no TCC.

Output is the wttr.in "format 3" — city + emoji + temperature on one
line. See <https://wttr.in> for full format options.
```

### `docs/reference/macos-cli-mapping.md`

Add a row under Tools:

```markdown
| Weather | `curl -s wttr.in/?format=3` |
```

## Step 8 — Update the home keyboard? No

In our example, we added a button under existing Tools. If we'd
created a brand-new category (e.g. `📡 Radio`):

- Edit `internal/telegram/keyboards/home.go` → `Categories` slice.
- Add a new namespace constant in `internal/telegram/callbacks/data.go`.
- Add a new entry in `internal/telegram/handlers/callbacks.go` switch.
- Create a new keyboard file `internal/telegram/keyboards/<ns>.go`.
- Create a new handler file `internal/telegram/handlers/<ns>.go`.

That's the full 11-file recipe (vs. 6 for adding to an existing
category).

## Run everything

```bash
make lint test build
```

Should all pass. If lint complains, run `make fmt` (gofumpt
auto-fix) and `make lint-fix` (golangci-lint auto-fix) for the
auto-correctable parts.

## Open the PR

```bash
git switch -c feat/weather
git add internal/domain/weather/ internal/telegram/keyboards/tls.go internal/telegram/handlers/tls.go internal/telegram/handlers/remaining_test.go internal/telegram/bot/bot.go cmd/macontrol/daemon.go docs/
git commit -m "feat(tools): add weather button via wttr.in"
git push -u origin feat/weather
gh pr create --title "feat(tools): add weather button via wttr.in" --body "$(cat <<'EOF'
## Summary
- New 🌤 Weather button under 🛠 Tools that returns a one-line weather summary
- Backing: curl wttr.in/?format=3 (no brew dep, no sudo, no TCC)

## Test plan
- [x] go test ./internal/domain/weather/...
- [x] go test ./internal/telegram/handlers/... -run TestTls_Weather
- [x] make lint
- [ ] Smoke-tested on real Mac — pending
EOF
)"
```

CI runs lint + tests. Once green, you (or a maintainer) merges.

## Common mistakes

- **Forgetting to register the namespace** — if it's a new category,
  the callback router won't find it. Symptom: tap does nothing.
- **Comment doesn't start with the identifier** — revive complains.
  Fix: `// Get returns…` not `// Returns…`.
- **Hard-coding the runner** — making the service take `*os/exec.Cmd`
  directly. Domain code MUST use `runner.Runner` so tests work.
- **Subprocess timeout too short** — if your action is intrinsically
  slow (e.g. a 30-second weather request), it'll hit the 15-second
  default timeout. Either use `context.WithTimeout` for a longer
  bound or document the limitation.
- **Leaking secrets in logs** — `slog.Debug("call", "url", url)`
  where url contains a token. Don't.

## Where to look for examples

Existing capabilities follow this exact pattern. Copy the closest
match:

- **Read-only with simple parsing**: `internal/domain/battery/battery.go`
- **State-changing with confirm**: `internal/domain/power/power.go`
- **Multi-step flow**: `internal/telegram/flows/joinwifi.go`
- **JSON parsing**: `internal/domain/bluetooth/bluetooth.go`
- **Conditional brew dep**: `internal/domain/notify/notify.go`

The codebase is small enough to grep for "Service struct" if you
want to scan all examples at once.
