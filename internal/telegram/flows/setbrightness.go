package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
)

// NewSetBrightness returns the typed-percentage [Flow] that
// sets display brightness to the user's exact value via
// [display.Service.Set].
//
// Behavior:
//   - Asks for an integer percentage 0..100. The flow speaks
//     percent; the service speaks fraction (0.0..1.0) — the
//     conversion happens in Handle.
//   - Re-prompts on parse failure or out-of-range value
//     without terminating.
//   - One-shot once a valid value is supplied: terminates
//     after the Set call (Done=true on success and failure).
//   - Reports the post-set level back to the user (which may
//     differ from the requested value if the brightness CLI
//     clamped or coerced).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleDisplay] when the user taps "Set exact
// value…".
func NewSetBrightness(svc *display.Service) Flow {
	return &setBrightnessFlow{svc: svc}
}

// setBrightnessFlow is the [NewSetBrightness]-returned [Flow].
// Holds only the [display.Service] reference; one-shot.
type setBrightnessFlow struct {
	svc *display.Service
}

// Name returns the dispatcher log identifier "dsp:set".
func (setBrightnessFlow) Name() string { return "dsp:set" }

// Start emits the typed-percentage prompt with the [0, 100]
// range hint.
func (setBrightnessFlow) Start(_ context.Context) Response {
	return Response{Text: "Enter brightness `0`-`100`. Reply `/cancel` to abort."}
}

// Handle parses the integer percentage and dispatches to
// [display.Service.Set] after converting percent to fraction.
//
// Routing rules (first match wins):
//  1. text fails to parse as int OR is < 0 OR > 100 → "Please
//     reply with a whole number between 0 and 100." (NOT
//     terminal — re-prompted).
//  2. Set returns non-nil err → "⚠ could not set brightness:
//     `<err>`" + Done.
//  3. Otherwise → "✅ Brightness set — `<post-set-pct>%`" +
//     Done. The reported pct is st.Level*100 from the service
//     response, NOT the user-requested value, so the user
//     sees what actually got applied.
func (f *setBrightnessFlow) Handle(ctx context.Context, text string) Response {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || v < 0 || v > 100 {
		return Response{Text: "Please reply with a whole number between 0 and 100."}
	}
	st, err := f.svc.Set(ctx, float64(v)/100)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not set brightness: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("✅ Brightness set — `%.0f%%`", st.Level*100),
		Done: true,
	}
}
