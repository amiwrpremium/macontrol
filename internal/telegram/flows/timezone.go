package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewTimezone returns the typed-name [Flow] that sets the
// system timezone via [tools.Service.TimezoneSet].
//
// Behavior:
//   - Asks for an IANA timezone string (e.g. "Europe/Istanbul"
//     or "UTC"). No client-side validation — invalid names
//     bubble up from the service's `systemsetup` call.
//   - One-shot: terminates after the Set call (Done=true on
//     both branches).
//   - On success, queries the current timezone back via
//     [tools.Service.TimezoneCurrent] and reports it. The
//     read-back error is silently dropped; the user always
//     sees a "✅ Timezone set" line even if the read fails
//     (cur is empty in that case).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleTools] when the user taps the typed-input
// "set timezone…" entry. The button-driven region/city picker
// uses a different code path entirely — see PR #57 for the
// picker rewrite.
func NewTimezone(svc *tools.Service) Flow { return &timezoneFlow{svc: svc} }

// timezoneFlow is the [NewTimezone]-returned [Flow]. Holds
// only the [tools.Service] reference; one-shot.
type timezoneFlow struct{ svc *tools.Service }

// Name returns the dispatcher log identifier "tls:tz".
func (timezoneFlow) Name() string { return "tls:tz" }

// Start emits the typed-input prompt with two example
// timezone names.
func (timezoneFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the timezone string (e.g. `Europe/Istanbul` or `UTC`). `/cancel` to abort."}
}

// Handle dispatches the user's text to
// [tools.Service.TimezoneSet], then reports the new current
// timezone back.
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty. Send a timezone name."
//     (NOT terminal — re-prompted).
//  2. TimezoneSet returns non-nil err → "⚠ set timezone
//     failed: `<err>`" + Done.
//  3. Otherwise → query [tools.Service.TimezoneCurrent]
//     (silently swallowing its error) and emit "✅ Timezone
//     set — `<cur>`" + Done. cur is empty when the read-back
//     fails; see the smells list.
func (f *timezoneFlow) Handle(ctx context.Context, text string) Response {
	tz := strings.TrimSpace(text)
	if tz == "" {
		return Response{Text: "Empty. Send a timezone name."}
	}
	if err := f.svc.TimezoneSet(ctx, tz); err != nil {
		return Response{Text: fmt.Sprintf("⚠ set timezone failed: `%v`", err), Done: true}
	}
	cur, _ := f.svc.TimezoneCurrent(ctx)
	return Response{Text: fmt.Sprintf("✅ Timezone set — `%s`", cur), Done: true}
}
