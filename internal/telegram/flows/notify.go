package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/notify"
)

// NewSendNotify returns the typed-text [Flow] that pushes a
// desktop notification banner via [notify.Service.Notify].
//
// Behavior:
//   - Parses the user reply as either "title | body", just a
//     title (no pipe → treated as body, blank title), or just
//     a body. Empty after both trims re-prompts.
//   - One-shot: terminates after the Notify call (Done=true on
//     both success and failure).
//   - Reports the underlying transport (terminal-notifier vs.
//     osascript fallback) on success so the operator can tell
//     which path was actually used.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleNotify] when the user taps "Send
// notification…".
func NewSendNotify(svc *notify.Service) Flow {
	return &sendNotifyFlow{svc: svc}
}

// sendNotifyFlow is the [NewSendNotify]-returned [Flow]. Holds
// only the [notify.Service] reference; one-shot.
type sendNotifyFlow struct{ svc *notify.Service }

// Name returns the dispatcher log identifier "ntf:send".
func (sendNotifyFlow) Name() string { return "ntf:send" }

// Start emits the typed-text prompt with the "title | body"
// hint.
func (sendNotifyFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the notification as `title | body`, or just a body. `/cancel` to abort."}
}

// Handle parses the title/body and dispatches to
// [notify.Service.Notify].
//
// Routing rules (first match wins):
//  1. Split text on the first `|` (using [strings.Cut]); trim
//     both sides.
//  2. body trims to empty → treat title as body and blank the
//     title (this is the "just typed a body, no pipe" case).
//  3. body still empty after that → "Empty notification. Try
//     again or `/cancel`." (NOT terminal — re-prompted).
//  4. Notify returns non-nil err → "⚠ notify failed: `<err>`"
//     + Done.
//  5. Otherwise → "✅ Notified via `<transport>`." + Done.
func (f *sendNotifyFlow) Handle(ctx context.Context, text string) Response {
	title, body, _ := strings.Cut(text, "|")
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if body == "" {
		body = title
		title = ""
	}
	if body == "" {
		return Response{Text: "Empty notification. Try again or `/cancel`."}
	}
	transport, err := f.svc.Notify(ctx, notify.Opts{Title: title, Body: body})
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ notify failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Notified via `%s`.", transport), Done: true}
}

// NewSay returns the typed-text [Flow] that speaks the user's
// reply via macOS's `say` CLI through [notify.Service.Say].
//
// Behavior:
//   - Validates the text isn't whitespace-only, then dispatches
//     immediately. No voice/rate selection in this flow — the
//     service uses the system default voice.
//   - One-shot: terminates after the Say call (Done=true on
//     both success and failure).
//   - The Say call is synchronous from the service's side: the
//     ✅ confirmation arrives after speech finishes.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleNotify] when the user taps "Say…".
func NewSay(svc *notify.Service) Flow { return &sayFlow{svc: svc} }

// sayFlow is the [NewSay]-returned [Flow]. Holds only the
// [notify.Service] reference; one-shot.
type sayFlow struct{ svc *notify.Service }

// Name returns the dispatcher log identifier "ntf:say".
func (sayFlow) Name() string { return "ntf:say" }

// Start emits the typed-text prompt for the speech body.
func (sayFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the text to speak. `/cancel` to abort."}
}

// Handle dispatches the user's text to [notify.Service.Say].
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty. Send the text or
//     `/cancel`." (NOT terminal — re-prompted).
//  2. Say returns non-nil err → "⚠ say failed: `<err>`" +
//     Done.
//  3. Otherwise → "✅ Spoken." + Done.
func (f *sayFlow) Handle(ctx context.Context, text string) Response {
	if strings.TrimSpace(text) == "" {
		return Response{Text: "Empty. Send the text or `/cancel`."}
	}
	if err := f.svc.Say(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ say failed: `%v`", err), Done: true}
	}
	return Response{Text: "✅ Spoken.", Done: true}
}
