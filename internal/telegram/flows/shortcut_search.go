package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// NewShortcutSearch returns the one-step typed-input [Flow]
// that filters the user's installed Apple Shortcuts by a
// substring and renders the filtered Run-Shortcut list as the
// flow's terminal response.
//
// Behavior:
//   - Belongs to the Shortcuts picker subsystem; reached when
//     the user taps "🔍 Search" on the shortcut-list keyboard.
//   - The substring is stashed in [callbacks.ShortMap] so
//     subsequent Prev/Next pagination keeps the filter via
//     callback args alone — no flow-instance state survives
//     past the terminal Done=true response.
//   - One-shot: terminates after rendering page 0 of the
//     filtered list. Pagination from there goes through the
//     handler's sc-page case, not back through this flow.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleTools] when the user taps "🔍 Search" on
// [keyboards.ToolsShortcutsList].
func NewShortcutSearch(svc *tools.Service, sm *callbacks.ShortMap) Flow {
	return &shortcutSearchFlow{svc: svc, sm: sm}
}

// shortcutSearchFlow is the [NewShortcutSearch]-returned
// [Flow]. Holds the service and the shared ShortMap.
//
// Field roles:
//   - svc is the [tools.Service] used by Handle to enumerate
//     installed shortcuts.
//   - sm is the shared [callbacks.ShortMap] — the same one
//     the bot passes to every handler, so the per-row ShortID
//     values emitted by Handle stay valid for subsequent
//     Prev/Next/run callbacks.
type shortcutSearchFlow struct {
	svc *tools.Service
	sm  *callbacks.ShortMap
}

// Name returns the dispatcher log identifier "tls:sc-search".
func (shortcutSearchFlow) Name() string { return "tls:sc-search" }

// Start emits the typed-input prompt with the case-insensitive
// hint.
func (shortcutSearchFlow) Start(_ context.Context) Response {
	return Response{
		Text: "🔍 Send a substring to filter your Shortcuts (case-insensitive). `/cancel` to abort.",
	}
}

// Handle filters the user's shortcuts by the supplied
// substring and renders the page-0 results.
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty filter — send some text or
//     `/cancel`." (NOT terminal — re-prompted).
//  2. [tools.Service.ShortcutsList] returns non-nil err → "⚠
//     couldn't list Shortcuts: `<err>`" + Done.
//  3. Otherwise: filter via [FilterShortcuts], stash sub in
//     ShortMap → filterID for pagination, render page 0 via
//     [keyboards.ToolsShortcutsList], emit (text, markup) +
//     Done=true.
func (f *shortcutSearchFlow) Handle(ctx context.Context, text string) Response {
	sub := strings.TrimSpace(text)
	if sub == "" {
		return Response{Text: "Empty filter — send some text or `/cancel`."}
	}
	all, err := f.svc.ShortcutsList(ctx)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ couldn't list Shortcuts: `%v`", err), Done: true}
	}
	matches := FilterShortcuts(all, sub)
	filterID := f.sm.Put(sub)
	items, totalPages := PageShortcuts(matches, 0, f.sm)
	headerText, kb := keyboards.ToolsShortcutsList(items, 0, totalPages, len(matches), filterID, sub)
	return Response{
		Text:   headerText,
		Markup: kb,
		Done:   true,
	}
}

// FilterShortcuts returns names from `all` whose
// case-insensitive form contains sub.
//
// Behavior:
//   - Empty sub returns the input slice unchanged (NOT a copy
//     — callers must not mutate the returned slice if they
//     also care about the input). The fast-path matters
//     because the no-filter case is the common case for the
//     pagination handler.
//   - Match is case-insensitive on both sides via
//     [strings.ToLower].
//
// Exported so the handler's pagination case can reuse the same
// matcher instead of duplicating the comparison logic.
func FilterShortcuts(all []string, sub string) []string {
	if sub == "" {
		return all
	}
	needle := strings.ToLower(sub)
	out := make([]string, 0, len(all))
	for _, n := range all {
		if strings.Contains(strings.ToLower(n), needle) {
			out = append(out, n)
		}
	}
	return out
}

// PageShortcuts slices the (possibly filtered) shortcut list
// to the requested 0-indexed page, registers fresh
// [callbacks.ShortMap] ids for the visible names, and returns
// the page items plus the total page count.
//
// Behavior:
//   - Page size is [keyboards.ShortcutsPageSize] (currently
//     15).
//   - Returns at least 1 totalPages even when all is empty,
//     so the keyboard always has something to render.
//   - Negative or out-of-range page values are clamped:
//     page < 0 → 0; page >= totalPages → totalPages-1.
//   - Empty slice case: start == end, items is empty (no
//     panic).
//   - Per-row label is truncated by
//     [keyboards.TruncateShortcutLabel] to fit the button
//     width.
//   - Per-row callback uses sm.Put(name) so the full
//     (possibly long) shortcut name round-trips through the
//     64-byte callback_data limit.
//
// Exported so the handler's pagination case can reuse the
// slicing + ShortMap registration logic.
func PageShortcuts(all []string, page int, sm *callbacks.ShortMap) ([]keyboards.ShortcutListItem, int) {
	total := len(all)
	totalPages := (total + keyboards.ShortcutsPageSize - 1) / keyboards.ShortcutsPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * keyboards.ShortcutsPageSize
	end := start + keyboards.ShortcutsPageSize
	if end > total {
		end = total
	}
	if start > end {
		start = end
	}
	items := make([]keyboards.ShortcutListItem, 0, end-start)
	for _, name := range all[start:end] {
		items = append(items, keyboards.ShortcutListItem{
			Label:   keyboards.TruncateShortcutLabel(name),
			ShortID: sm.Put(name),
		})
	}
	return items, totalPages
}
