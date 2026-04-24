package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/media"
)

// NewRecord returns the typed-duration [Flow] that records the
// screen for the given number of seconds via
// [media.Service.Record], then hands the resulting file path
// to the supplied sendVideo callback.
//
// Arguments:
//   - svc is the [media.Service] doing the actual capture
//     (shells out to screencapture).
//   - chatID is captured for symmetry with other constructors
//     and future logging — currently unused inside the flow
//     because sendVideo already closes over the chat target.
//   - sendVideo is the per-chat upload-and-cleanup callback.
//     The callee owns the file at path: must upload and
//     remove. Returning an error from sendVideo is reported to
//     the user and terminates the flow; the file is left in
//     place if it has not yet been deleted (no compensating
//     cleanup here — see the smells list).
//
// Behavior:
//   - Asks for an integer second count, validates it's within
//     [1, 120], then dispatches.
//   - Re-prompts on parse failure or out-of-range value
//     without terminating the flow.
//   - One-shot once dispatch happens: terminates after the
//     send callback returns (Done=true on success and on
//     either failure path).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleMedia] when the user taps "Record…".
func NewRecord(svc *media.Service, chatID int64, sendVideo func(ctx context.Context, path string) error) Flow {
	return &recordFlow{svc: svc, chatID: chatID, send: sendVideo}
}

// recordFlow is the [NewRecord]-returned [Flow]. Holds the
// service plus the per-chat send callback.
//
// Field roles:
//   - svc is the [media.Service] doing the recording.
//   - chatID is informational; not used inside Handle.
//   - send is the sendVideo callback the constructor was
//     given. Owns the file at path on call.
type recordFlow struct {
	svc    *media.Service
	chatID int64
	send   func(ctx context.Context, path string) error
}

// Name returns the dispatcher log identifier "med:record".
func (recordFlow) Name() string { return "med:record" }

// Start emits the typed-duration prompt with the [1, 120]
// range hint.
func (recordFlow) Start(_ context.Context) Response {
	return Response{Text: "Record for how many seconds? (1-120). Reply `/cancel` to abort."}
}

// Handle parses the second count, dispatches to
// [media.Service.Record], then forwards the file via the
// constructor's send callback.
//
// Routing rules (first match wins):
//  1. text fails to parse as int OR is < 1 OR > 120 → "Please
//     reply with an integer between 1 and 120." (NOT terminal
//     — user re-prompted).
//  2. ctx deadline is sooner than secs+30 from now → swap ctx
//     for a fresh background-rooted context with a secs+30
//     timeout. The +30s budgets the upload after the
//     recording finishes; without this, an early-deadline
//     parent (e.g. a 30s handler ctx) would cut the recording
//     short.
//  3. Record returns non-nil err → "⚠ record failed: `<err>`"
//     + Done. The file (if any) is owned by media.Service and
//     its disposition on error is the service's contract, not
//     this flow's.
//  4. send returns non-nil err → "⚠ upload failed: `<err>`" +
//     Done. The file at path is NOT cleaned up on this path —
//     send owns it once called (see the smells list for the
//     leak).
//  5. Otherwise → "✅ Recorded <N>s." + Done.
func (f *recordFlow) Handle(ctx context.Context, text string) Response {
	secs, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || secs < 1 || secs > 120 {
		return Response{Text: "Please reply with an integer between 1 and 120."}
	}
	deadline, _ := ctx.Deadline()
	if deadline.Before(time.Now().Add(time.Duration(secs+30) * time.Second)) {
		c, cancel := context.WithTimeout(context.Background(), time.Duration(secs+30)*time.Second)
		defer cancel()
		ctx = c
	}
	path, err := f.svc.Record(ctx, time.Duration(secs)*time.Second)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ record failed: `%v`", err), Done: true}
	}
	if err := f.send(ctx, path); err != nil {
		return Response{Text: fmt.Sprintf("⚠ upload failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Recorded %ds.", secs), Done: true}
}
