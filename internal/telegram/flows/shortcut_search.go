package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// NewShortcutSearch returns a one-step flow that asks the user for a
// substring, then renders the filtered Run-Shortcut list as the
// flow's terminal response. The substring is stashed in the
// ShortMap so subsequent Prev/Next pagination can preserve the
// filter via callback args alone.
func NewShortcutSearch(svc *tools.Service, sm *callbacks.ShortMap) Flow {
	return &shortcutSearchFlow{svc: svc, sm: sm}
}

type shortcutSearchFlow struct {
	svc *tools.Service
	sm  *callbacks.ShortMap
}

func (shortcutSearchFlow) Name() string { return "tls:sc-search" }

func (shortcutSearchFlow) Start(_ context.Context) Response {
	return Response{
		Text: "🔍 Send a substring to filter your Shortcuts (case-insensitive). `/cancel` to abort.",
	}
}

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

// FilterShortcuts returns names containing sub (case-insensitive).
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

// PageShortcuts slices the (possibly filtered) list to the requested
// 0-indexed page, registers fresh ShortMap ids for the visible names,
// and returns the page items plus the total page count (≥1).
// Exported so the handler's pagination case can reuse the slicing
// + ShortMap registration logic.
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
