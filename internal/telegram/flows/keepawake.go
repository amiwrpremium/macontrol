package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/power"
)

// NewKeepAwake returns the typed-duration [Flow] that starts
// `caffeinate -d -t <seconds>` via [power.Service.KeepAwake].
//
// Behavior:
//   - Asks for an integer minute count, validates it's within
//     [1, 1440] (1 minute to 24 hours), then dispatches.
//   - Re-prompts on parse failure or out-of-range value
//     without terminating the flow (the user gets to retry).
//   - Terminates after a successful KeepAwake call OR a
//     failure from the service (Done=true on both).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handlePower] when the user taps "Keep awake…".
func NewKeepAwake(svc *power.Service) Flow {
	return &keepAwakeFlow{svc: svc}
}

// keepAwakeFlow is the [NewKeepAwake]-returned [Flow]. Holds
// only the [power.Service] reference; the duration is parsed
// from each Handle call so no per-step state is kept.
type keepAwakeFlow struct{ svc *power.Service }

// Name returns the dispatcher log identifier "pwr:keepawake".
func (keepAwakeFlow) Name() string { return "pwr:keepawake" }

// Start emits the typed-duration prompt. Called once when the
// flow is installed.
func (keepAwakeFlow) Start(_ context.Context) Response {
	return Response{Text: "Keep awake for how many minutes? (1-1440). Reply `/cancel` to abort."}
}

// Handle parses the integer minute count and dispatches to
// [power.Service.KeepAwake].
//
// Routing rules (first match wins):
//  1. text fails to parse as int OR is < 1 OR > 1440 →
//     "Please reply with an integer between 1 and 1440."
//     (NOT terminal — user re-prompted).
//  2. KeepAwake returns non-nil err → "⚠ could not start
//     keep-awake: `<err>`" + Done.
//  3. otherwise → "☕ Keep-awake running for <N> min." + Done.
//
// The 1440-minute cap is a safety rail: nothing in caffeinate
// itself enforces it, but unbounded sleep-prevention from a
// chat command is a recipe for "I forgot to cancel and burnt
// my battery overnight".
func (f *keepAwakeFlow) Handle(ctx context.Context, text string) Response {
	minutes, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || minutes < 1 || minutes > 1440 {
		return Response{Text: "Please reply with an integer between 1 and 1440."}
	}
	if err := f.svc.KeepAwake(ctx, time.Duration(minutes)*time.Minute); err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not start keep-awake: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("☕ Keep-awake running for %d min.", minutes),
		Done: true,
	}
}
