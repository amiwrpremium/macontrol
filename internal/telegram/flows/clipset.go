package flows

import (
	"context"
	"fmt"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewClipSet returns the typed-text [Flow] that pipes the
// next user message into [tools.Service.ClipboardWrite].
//
// Behavior:
//   - The returned flow has no validation step: any non-empty
//     reply text is treated as the clipboard payload, including
//     multi-line bodies. Telegram's plain-text limit (4 KiB)
//     is the only upper bound.
//   - One-shot: terminates after a single user reply (success
//     or failure both set Done=true).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleTools] when the user taps "Set clipboard…".
func NewClipSet(svc *tools.Service) Flow { return &clipSetFlow{svc: svc} }

// clipSetFlow is the [NewClipSet]-returned [Flow]. Holds only
// the [tools.Service] reference; no per-step state since the
// flow is one-shot.
type clipSetFlow struct{ svc *tools.Service }

// Name returns the dispatcher log identifier "tls:clipset",
// matching the namespace+action pair the handler uses to
// install this flow.
func (clipSetFlow) Name() string { return "tls:clipset" }

// Start emits the typed-input prompt instructing the user how
// to abort. Called once when the flow is installed.
func (clipSetFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the text you want on the clipboard. `/cancel` to abort."}
}

// Handle writes the user's text to the clipboard via
// [tools.Service.ClipboardWrite] and returns a terminal
// Response (Done=true on both branches).
//
// Routing rules (first match wins):
//  1. ClipboardWrite returns non-nil err →
//     "⚠ clipboard write failed: `<err>`" + Done.
//  2. Otherwise → "✅ Clipboard updated." + Done.
func (f *clipSetFlow) Handle(ctx context.Context, text string) Response {
	if err := f.svc.ClipboardWrite(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ clipboard write failed: `%v`", err), Done: true}
	}
	return Response{Text: "✅ Clipboard updated.", Done: true}
}
