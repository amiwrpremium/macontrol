package keyboards

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// ShortcutsPageSize controls how many shortcut buttons fit on one
// list page. 15 keeps the message comfortable on a phone screen
// while staying well under Telegram's hard ~100-button limit.
const ShortcutsPageSize = 15

// TimezonesPageSize matches ShortcutsPageSize for the city picker.
const TimezonesPageSize = 15

// TimezoneRegion is one row on the timezone region-picker page.
type TimezoneRegion struct {
	Slug  string // bare region name "Africa", "America", etc.
	Count int    // how many timezones live under this region
}

// TimezoneListItem mirrors ShortcutListItem: a pre-built display
// label (city + flag emoji when known) plus the ShortMap id that
// resolves to the full IANA timezone name.
type TimezoneListItem struct {
	Label   string
	ShortID string
}

// TimezoneTopLevel is one entry on the region-picker page for
// timezones with no '/' (GMT, UTC, etc.) — they get rendered
// inline alongside the region buttons.
type TimezoneTopLevel struct {
	Label   string // typically just the timezone name itself
	ShortID string
}

// ShortcutListItem is one entry on the Run Shortcut list page.
// Label is what the button shows (already truncated for display);
// ShortID is the ShortMap-issued opaque id resolving to the full
// name on tap.
type ShortcutListItem struct {
	Label   string
	ShortID string
}

// ToolsDiskRow is one entry on the Disks list page. ShortID is a
// callbacks.ShortMap-issued opaque id for the mount path so we don't
// blow the 64-byte callback_data limit on long /Volumes/ paths.
type ToolsDiskRow struct {
	Mount    string // for the button label
	Size     string // human form, e.g. "460Gi"
	Capacity string // e.g. "54%"
	ShortID  string // map id resolved by handler
}

// Tools renders the 🛠 Tools menu. features gates the Shortcuts runner
// (needs macOS 13+).
func Tools(features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	text = "🛠 *Tools*"
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "📋 Clipboard (read)", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "get")},
			{Text: "📋 Clipboard (set)…", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "set")},
		},
		{
			{Text: "🧭 Timezone…", CallbackData: callbacks.Encode(callbacks.NSTools, "tz")},
			{Text: "🔄 Sync time", CallbackData: callbacks.Encode(callbacks.NSTools, "synctime")},
		},
		{
			{Text: "💿 Disks", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
		},
	}
	if features.Shortcuts {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "⚡ Run Shortcut…", CallbackData: callbacks.Encode(callbacks.NSTools, "shortcut")},
		})
	}
	rows = append(rows, NavWithBack(callbacks.NSNav, "home"))
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// ToolsDisksList renders the 💿 Disks list page: one button per
// user-facing mount (label "<mount> · <size> · <cap> used"), then
// the standard Refresh / ← Back / 🏠 Home rows. Tap drills into
// ToolsDiskPanel via tls:disk:<shortID>.
func ToolsDisksList(rows []ToolsDiskRow) *models.InlineKeyboardMarkup {
	out := make([][]models.InlineKeyboardButton, 0, len(rows)+2)
	for _, d := range rows {
		out = append(out, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s · %s · %s used", d.Mount, d.Size, d.Capacity),
			CallbackData: callbacks.Encode(callbacks.NSTools, "disk", d.ShortID),
		}})
	}
	out = append(out, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSTools, "open")},
	})
	out = append(out, Nav())
	return &models.InlineKeyboardMarkup{InlineKeyboard: out}
}

// ToolsShortcutsList renders the paginated Run Shortcut list page.
//
//   - items     — the page slice (already paginated by the caller).
//   - page      — 0-indexed page number.
//   - totalPages — total page count (≥1).
//   - total     — count of shortcuts AFTER any filter (for the header).
//   - filterID  — empty for unfiltered; a ShortMap id when a search
//     is active. Carried in Prev/Next callback args so paging
//     preserves the filter.
//   - filterTerm — the original substring shown verbatim in the
//     header when filtered. Empty when unfiltered.
//
// The returned text is the dashboard header; markup is the paginated
// keyboard with Prev/Next, Search, Type-exact-name, Refresh, Back,
// Home rows below the per-shortcut buttons.
func ToolsShortcutsList(items []ShortcutListItem, page, totalPages, total int, filterID, filterTerm string) (text string, markup *models.InlineKeyboardMarkup) {
	header := fmt.Sprintf("⚡ *Run Shortcut*  ·  Page %d/%d  ·  %d shortcuts",
		page+1, atLeastOne(totalPages), total)
	if filterTerm != "" {
		header = fmt.Sprintf("⚡ *Run Shortcut*  ·  Page %d/%d  ·  Filtered: `%s` · %d match%s",
			page+1, atLeastOne(totalPages), filterTerm, total, plural(total, "es"))
	}
	if total == 0 {
		header += "\n\n_No shortcuts found._"
	}
	text = header

	rows := make([][]models.InlineKeyboardButton, 0, len(items)+4)
	for _, it := range items {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         it.Label,
			CallbackData: callbacks.Encode(callbacks.NSTools, "sc-run", it.ShortID, strconv.Itoa(page), filterIDArg(filterID)),
		}})
	}

	// Pagination row — only shown when there's more than one page.
	if totalPages > 1 {
		nav := make([]models.InlineKeyboardButton, 0, 2)
		if page > 0 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "← Prev",
				CallbackData: callbacks.Encode(callbacks.NSTools, "sc-page", strconv.Itoa(page-1), filterIDArg(filterID)),
			})
		}
		if page < totalPages-1 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "Next →",
				CallbackData: callbacks.Encode(callbacks.NSTools, "sc-page", strconv.Itoa(page+1), filterIDArg(filterID)),
			})
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}

	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔍 Search", CallbackData: callbacks.Encode(callbacks.NSTools, "sc-search")},
		{Text: "⌨ Type exact name", CallbackData: callbacks.Encode(callbacks.NSTools, "sc-type")},
	})
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "shortcut")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSTools, "open")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// ToolsTimezoneRegions renders step 1 of the timezone picker: one
// button per region, plus any top-level timezones (GMT, UTC) inline
// below. No pagination — there are only ~12 regions.
//
// `current` is the currently-set timezone (shown in the header so the
// user can see what they're changing from).
func ToolsTimezoneRegions(current string, regions []TimezoneRegion, topLevels []TimezoneTopLevel) (text string, markup *models.InlineKeyboardMarkup) {
	header := "🧭 *Set timezone*"
	if current != "" {
		header = fmt.Sprintf("🧭 *Set timezone*  ·  Current: `%s`", current)
	}
	text = header

	rows := make([][]models.InlineKeyboardButton, 0, len(regions)+len(topLevels)+3)
	for _, r := range regions {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s (%d)", r.Slug, r.Count),
			CallbackData: callbacks.Encode(callbacks.NSTools, "tz-region", r.Slug),
		}})
	}
	for _, tl := range topLevels {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         tl.Label,
			CallbackData: callbacks.Encode(callbacks.NSTools, "tz-set", tl.ShortID),
		}})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "⌨ Type exact name", CallbackData: callbacks.Encode(callbacks.NSTools, "tz-type")},
	})
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "tz")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSTools, "open")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// ToolsTimezoneCities renders step 2 of the timezone picker: a
// paginated list of cities within a region. Each row is a tappable
// button that applies the timezone via tz-set. Search and Type-exact
// fallbacks live below the page nav. Filter (when active) carries
// through Prev/Next via filterID, same pattern as ToolsShortcutsList.
func ToolsTimezoneCities(region, current string, items []TimezoneListItem, page, totalPages, total int, filterID, filterTerm string) (text string, markup *models.InlineKeyboardMarkup) {
	header := fmt.Sprintf("🧭 *%s*  ·  Page %d/%d  ·  %d timezone%s",
		region, page+1, atLeastOne(totalPages), total, plural(total, "s"))
	if filterTerm != "" {
		header = fmt.Sprintf("🧭 *%s*  ·  Page %d/%d  ·  Filtered: `%s` · %d match%s",
			region, page+1, atLeastOne(totalPages), filterTerm, total, plural(total, "es"))
	}
	if current != "" {
		header += fmt.Sprintf("  ·  Current: `%s`", current)
	}
	if total == 0 {
		header += "\n\n_No timezones found._"
	}
	text = header

	rows := make([][]models.InlineKeyboardButton, 0, len(items)+5)
	for _, it := range items {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         it.Label,
			CallbackData: callbacks.Encode(callbacks.NSTools, "tz-set", it.ShortID),
		}})
	}
	if totalPages > 1 {
		nav := make([]models.InlineKeyboardButton, 0, 2)
		if page > 0 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "← Prev",
				CallbackData: callbacks.Encode(callbacks.NSTools, "tz-page", region, strconv.Itoa(page-1), filterIDArg(filterID)),
			})
		}
		if page < totalPages-1 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "Next →",
				CallbackData: callbacks.Encode(callbacks.NSTools, "tz-page", region, strconv.Itoa(page+1), filterIDArg(filterID)),
			})
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔍 Search", CallbackData: callbacks.Encode(callbacks.NSTools, "tz-search", region)},
		{Text: "⌨ Type exact name", CallbackData: callbacks.Encode(callbacks.NSTools, "tz-type")},
	})
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "← Back to regions", CallbackData: callbacks.Encode(callbacks.NSTools, "tz")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// FlagFromISO2 converts a 2-letter ISO 3166-1 alpha-2 country code
// to a flag emoji using regional indicator symbols (each letter
// becomes its corresponding 1F1E6–1F1FF code point). Returns "" for
// non-2-char input or codes outside A–Z.
func FlagFromISO2(code string) string {
	if len(code) != 2 {
		return ""
	}
	var r [2]rune
	for i, c := range code {
		if c < 'A' || c > 'Z' {
			return ""
		}
		r[i] = 0x1F1E6 + rune(c-'A')
	}
	return string(r[:])
}

// TruncateShortcutLabel shortens a shortcut name to fit on a button
// row; the full name is preserved in ShortMap. Rune-aware so
// multi-byte names don't get cut mid-character.
func TruncateShortcutLabel(name string) string {
	const maxLen = 40
	if utf8.RuneCountInString(name) <= maxLen {
		return name
	}
	runes := []rune(name)
	return string(runes[:maxLen-1]) + "…"
}

// filterIDArg returns "-" for an empty filter so the callback always
// has a fixed positional arg layout. Handlers translate "-" back to
// "" before lookup.
func filterIDArg(filterID string) string {
	if filterID == "" {
		return "-"
	}
	return filterID
}

// atLeastOne returns 1 when n is below 1; used for "Page X/Y"
// headers so we never render "Page 1/0".
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func plural(n int, suffix string) string {
	if n == 1 {
		return ""
	}
	return suffix
}

// ToolsDiskPanel renders the per-disk drill-down. Open in Finder is
// always shown; Eject is gated on removable (only safe for
// /Volumes/* with Removable Media: Removable). Refresh re-runs the
// drill-down for this disk; Back returns to the disks list.
func ToolsDiskPanel(shortID string, removable bool) *models.InlineKeyboardMarkup {
	actions := []models.InlineKeyboardButton{
		{Text: "📂 Open in Finder", CallbackData: callbacks.Encode(callbacks.NSTools, "disk-open", shortID)},
	}
	if removable {
		actions = append(actions, models.InlineKeyboardButton{
			Text: "⏏ Eject", CallbackData: callbacks.Encode(callbacks.NSTools, "disk-eject", shortID),
		})
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			actions,
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "disk", shortID)},
				{Text: "← Back to Disks", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
			},
			Nav(),
		},
	}
}
