package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
)

// NewSetVolume returns the typed-percentage [Flow] that sets
// output volume to the user's exact value via
// [sound.Service.Set].
//
// Behavior:
//   - Asks for an integer percentage 0..100. Sound's API
//     speaks percent natively (unlike [display]) so no
//     conversion is needed in Handle.
//   - Re-prompts on parse failure or out-of-range value
//     without terminating.
//   - One-shot once a valid value is supplied: terminates
//     after the Set call (Done=true on success and failure).
//   - Reports both the post-set level AND the muted flag —
//     muted: true after a value-set is unusual but possible
//     (set to 0 may also flip muted on some macOS versions).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleSound] when the user taps "Set exact
// value…".
func NewSetVolume(svc *sound.Service) Flow {
	return &setVolumeFlow{svc: svc}
}

// setVolumeFlow is the [NewSetVolume]-returned [Flow]. Holds
// only the [sound.Service] reference; one-shot.
type setVolumeFlow struct {
	svc *sound.Service
}

// Name returns the dispatcher log identifier "snd:set".
func (setVolumeFlow) Name() string { return "snd:set" }

// Start emits the typed-percentage prompt with the [0, 100]
// range hint.
func (setVolumeFlow) Start(_ context.Context) Response {
	return Response{Text: "Enter target volume (`0`-`100`). Reply `/cancel` to abort."}
}

// Handle parses the integer percentage and dispatches to
// [sound.Service.Set].
//
// Routing rules (first match wins):
//  1. text fails to parse as int OR is < 0 OR > 100 → "Please
//     reply with a whole number between 0 and 100." (NOT
//     terminal — re-prompted).
//  2. Set returns non-nil err → "⚠ could not set volume:
//     `<err>`" + Done.
//  3. Otherwise → "✅ Volume set — `<pct>%` · muted: `<bool>`"
//     + Done. Both fields come from the service response so
//     the user sees what actually got applied.
func (f *setVolumeFlow) Handle(ctx context.Context, text string) Response {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || v < 0 || v > 100 {
		return Response{Text: "Please reply with a whole number between 0 and 100."}
	}
	st, err := f.svc.Set(ctx, v)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not set volume: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("✅ Volume set — `%d%%` · muted: `%t`", st.Level, st.Muted),
		Done: true,
	}
}
