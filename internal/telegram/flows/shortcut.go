package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewShortcut returns the typed-name [Flow] that runs an
// Apple Shortcut by name via [tools.Service.ShortcutRun].
//
// Behavior:
//   - Asks for a Shortcut name (case-sensitive — that's the
//     Shortcuts CLI contract, not a flow choice).
//   - One-shot: terminates after the run attempt (Done=true on
//     both success and failure).
//   - The run is fire-and-forget from the user's perspective:
//     ShortcutRun returns once the shortcut process exits, but
//     the shortcut may have side effects that outlive it.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleTools] when the user picks the typed-input
// "run shortcut by name…" entry. The interactive picker
// (alphabetical browse + search) uses a different code path
// — this typed flow is the power-user fallback for known names.
func NewShortcut(svc *tools.Service) Flow { return &shortcutFlow{svc: svc} }

// shortcutFlow is the [NewShortcut]-returned [Flow]. Holds
// only the [tools.Service] reference; one-shot.
type shortcutFlow struct{ svc *tools.Service }

// Name returns the dispatcher log identifier "tls:shortcut".
func (shortcutFlow) Name() string { return "tls:shortcut" }

// Start emits the typed-input prompt. The "case-sensitive"
// hint is important — the macOS Shortcuts CLI rejects
// mismatched case with an unhelpful error.
func (shortcutFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the Shortcut name (case-sensitive). `/cancel` to abort."}
}

// Handle dispatches the user's text to
// [tools.Service.ShortcutRun].
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty. Send a Shortcut name."
//     (NOT terminal — re-prompted).
//  2. ShortcutRun returns non-nil err → "⚠ run failed:
//     `<err>`" + Done.
//  3. Otherwise → "✅ Ran `<name>`." + Done. The success
//     message confirms the CLI exited 0; it does NOT confirm
//     the shortcut achieved its intended side effect.
func (f *shortcutFlow) Handle(ctx context.Context, text string) Response {
	name := strings.TrimSpace(text)
	if name == "" {
		return Response{Text: "Empty. Send a Shortcut name."}
	}
	if err := f.svc.ShortcutRun(ctx, name); err != nil {
		return Response{Text: fmt.Sprintf("⚠ run failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Ran `%s`.", name), Done: true}
}
