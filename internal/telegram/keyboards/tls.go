package keyboards

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// ShortcutsPageSize is how many shortcut buttons fit on one
// list page in [ToolsShortcutsList]. 15 keeps the message
// comfortable on a phone screen while staying well under
// Telegram's hard ~100-button limit.
const ShortcutsPageSize = 15

// TimezonesPageSize is how many timezone buttons fit on one
// city-picker page in [ToolsTimezoneCities]. Matches
// [ShortcutsPageSize] for visual consistency.
const TimezonesPageSize = 15

// TimezoneRegion is one button on the timezone region-picker
// page rendered by [ToolsTimezoneRegions].
//
// Field roles:
//   - Slug is the bare region name (the part of an IANA name
//     before the first '/'), e.g. "Africa", "America", "Asia".
//     Used both as the button label prefix and as the callback
//     arg for `tz-region`.
//   - Count is how many timezones live under this region. Shown
//     in parens after the slug ("Africa (52)") so the user can
//     gauge the page count.
type TimezoneRegion struct {
	// Slug is the bare region name, e.g. "Africa" or "America".
	Slug string

	// Count is how many timezones live under this region.
	// Rendered after the slug in the button label.
	Count int
}

// TimezoneListItem is one button on the timezone city-picker
// page rendered by [ToolsTimezoneCities]. Mirrors the shape of
// [ShortcutListItem] — see that type for the ShortMap rationale.
//
// Field roles:
//   - Label is the pre-built display string (city name + flag
//     emoji prefix when known), e.g. "🇺🇸 Los_Angeles".
//   - ShortID is the [callbacks.ShortMap]-issued opaque id
//     resolving to the full IANA timezone name on tap.
type TimezoneListItem struct {
	// Label is the button text shown to the user.
	Label string

	// ShortID is the ShortMap-issued opaque id that resolves to
	// the full IANA timezone name on tap.
	ShortID string
}

// TimezoneTopLevel is one entry on the region-picker page for
// timezones that have no '/' (GMT, UTC, etc.). They get rendered
// inline alongside the region buttons rather than as their own
// "region" with one timezone.
//
// Field roles:
//   - Label is the button text, typically the bare timezone
//     name itself ("GMT").
//   - ShortID is the ShortMap-issued opaque id that resolves to
//     the timezone name on tap (so [ToolsTimezoneCities]'s
//     `tz-set` callback shape can be reused unchanged).
type TimezoneTopLevel struct {
	// Label is the button text, typically the bare timezone
	// name itself (e.g. "GMT").
	Label string

	// ShortID is the ShortMap-issued opaque id resolving to
	// the timezone name on tap.
	ShortID string
}

// ShortcutListItem is one entry on the Run Shortcut list page
// rendered by [ToolsShortcutsList].
//
// Why ShortID instead of the full name: shortcuts can be very
// long ("Send 'Be right back' to last chat with friend X") and
// would blow Telegram's 64-byte callback_data budget once paired
// with the namespace + action + page number. The keyboard parks
// the full name in the [callbacks.ShortMap] and embeds only the
// short opaque id.
//
// Field roles:
//   - Label is the already-truncated display string from
//     [TruncateShortcutLabel].
//   - ShortID is the ShortMap-issued opaque id that resolves to
//     the full shortcut name on tap.
type ShortcutListItem struct {
	// Label is the already-truncated display name shown on the
	// button.
	Label string

	// ShortID is the ShortMap-issued opaque id that resolves to
	// the full shortcut name on tap.
	ShortID string
}

// ToolsDiskRow is one button on the Disks list page rendered by
// [ToolsDisksList]. The mount path goes through [callbacks.ShortMap]
// because long /Volumes/Foo Bar Baz/ paths would blow the 64-byte
// callback_data limit.
//
// Field roles:
//   - Mount is the mount point shown in the button label
//     (verbatim, including any spaces).
//   - Size is the human-readable total size, e.g. "460Gi".
//   - Capacity is the percent-used string, e.g. "54%".
//   - ShortID is the opaque id resolved by the handler back to
//     the mount path on tap.
type ToolsDiskRow struct {
	// Mount is the mount point shown in the button label.
	Mount string

	// Size is the human-readable total size, e.g. "460Gi".
	Size string

	// Capacity is the percent-used string, e.g. "54%".
	Capacity string

	// ShortID is the [callbacks.ShortMap]-issued opaque id
	// resolved by the handler back to the mount path.
	ShortID string
}

// Tools renders the 🛠 Tools dashboard. features gates the
// Shortcuts runner button — it's only shown when the
// [capability.Features.Shortcuts] flag (macOS 13+) is set.
//
// Behavior:
//   - Returns the static "🛠 *Tools*" header text.
//   - Builds a 5-row keyboard: Clipboard read/set pair,
//     Timezone and Sync-time pair, Disks alone, optional
//     Shortcuts row, Back/Home nav.
//   - The Disks row is alone (not paired) for visual weight —
//     it's the only "stateful drill-down" on this dashboard.
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
// user-facing mount, then the standard Refresh + ← Back + 🏠 Home
// nav rows.
//
// Behavior:
//   - Each disk button label is "<mount> · <size> · <cap> used",
//     with the callback data carrying the mount's [ShortMap] id
//     under the `tls:disk:<shortID>` shape.
//   - On tap, the handler dispatches into [ToolsDiskPanel] for
//     the per-disk drill-down.
//   - Refresh re-renders this list (callback `tls:disks`); Back
//     returns to the Tools dashboard (callback `tls:open`).
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

// ToolsShortcutsList renders the paginated Run Shortcut list
// page.
//
// Arguments:
//   - items: the page slice (already paginated by the caller —
//     this function does NOT slice).
//   - page: 0-indexed page number for the header.
//   - totalPages: total page count (always rendered as ≥1 via
//     [atLeastOne] so an empty list still shows "Page 1/1").
//   - total: count of shortcuts AFTER any filter, shown in the
//     header.
//   - filterID: the [callbacks.ShortMap] id of the active
//     filter substring, "" when unfiltered. Carried in
//     Prev/Next callback args via [filterIDArg] so paging
//     preserves the filter.
//   - filterTerm: the original filter string shown verbatim in
//     the header. Empty when unfiltered.
//
// Behavior:
//   - Builds a header that switches between the "N shortcuts"
//     and "Filtered: <term> · N matches" variants based on
//     filterTerm.
//   - When total == 0, appends "_No shortcuts found._" to the
//     header.
//   - Builds one tappable row per item (callback `tls:sc-run`
//     carrying the item's ShortID, the page index, and the
//     filterID — so a future cancel of the running shortcut
//     could navigate the user back to exactly the page they
//     were on).
//   - Adds Prev/Next nav row only when totalPages > 1, and only
//     includes the relevant arrow per page edge.
//   - Adds Search + Type-exact-name row, Refresh + Back row,
//     and the standard 🏠 Home row.
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

// ToolsTimezoneRegions renders step 1 of the timezone picker:
// one button per IANA region (Africa / America / Asia / …) plus
// any top-level timezones (GMT / UTC) inline below.
//
// Arguments:
//   - current: the currently-set IANA timezone, shown verbatim
//     in the header so the user sees what they're changing
//     from. Empty disables the "Current: …" suffix.
//   - regions: the per-region tile data. Order is preserved by
//     the renderer.
//   - topLevels: top-level timezones (no '/' in their IANA
//     name). Rendered as their own buttons after the regions.
//
// Behavior:
//   - No pagination — there are only ~12 regions plus a handful
//     of top-levels.
//   - Each region button's label is "<slug> (<count>)"; on tap
//     dispatches into [ToolsTimezoneCities] via `tls:tz-region`.
//   - Top-level buttons skip the city picker and apply directly
//     via `tls:tz-set` (same shape city buttons use).
//   - Adds the typed-name fallback (`tls:tz-type`), Refresh +
//     Back row, and the standard Home row.
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
// paginated list of cities within one region.
//
// Arguments mirror [ToolsShortcutsList]: items / page /
// totalPages / total / filterID / filterTerm. The extra `region`
// + `current` arguments are specific to the timezone picker
// (region for the header + page-callback arg, current for the
// "Current: …" header suffix).
//
// Behavior:
//   - Each city button's callback is `tls:tz-set:<shortID>` —
//     same shape as the top-level entries in
//     [ToolsTimezoneRegions], so the handler doesn't need a
//     separate code path for "tapped from the city list".
//   - Header switches between "N timezones" and the filtered
//     "Filtered: <term> · N matches" variant based on
//     filterTerm.
//   - Pagination row uses `tls:tz-page` (carrying the region
//     slug, page index, and filterID) so paging within a
//     region preserves the filter.
//   - Search row scopes to the current region via
//     `tls:tz-search:<region>`; type-exact uses the same
//     unscoped `tls:tz-type` as the region picker.
//   - Back goes to "tz" (the region picker), not "open" (the
//     Tools dashboard) — drill-back, not exit.
func ToolsTimezoneCities(region, current string, items []TimezoneListItem, page, totalPages, total int, filterID, filterTerm string) (text string, markup *models.InlineKeyboardMarkup) {
	text = tzCitiesHeader(region, current, page, totalPages, total, filterTerm)
	rows := make([][]models.InlineKeyboardButton, 0, len(items)+5)
	for _, it := range items {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         it.Label,
			CallbackData: callbacks.Encode(callbacks.NSTools, "tz-set", it.ShortID),
		}})
	}
	if nav := tzCitiesPagerRow(region, page, totalPages, filterID); nav != nil {
		rows = append(rows, nav)
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

// tzCitiesHeader builds the header line for the city picker.
// Picks between the unfiltered "N timezones" and the filtered
// "Filtered: <term> · N matches" variant, and appends the
// "Current: …" suffix when known. Renders an empty-state
// trailer when total == 0.
func tzCitiesHeader(region, current string, page, totalPages, total int, filterTerm string) string {
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
	return header
}

// tzCitiesPagerRow builds the Prev/Next pagination row, or
// returns nil when totalPages <= 1 (no pagination needed).
func tzCitiesPagerRow(region string, page, totalPages int, filterID string) []models.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
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
	if len(nav) == 0 {
		return nil
	}
	return nav
}

// FlagFromISO2 converts a 2-letter ISO 3166-1 alpha-2 country
// code to its flag emoji using regional indicator symbols.
//
// Behavior:
//   - Validates len(code) == 2; returns "" otherwise.
//   - Validates each rune is uppercase A-Z; returns "" on any
//     other character (lowercase, digits, multi-byte).
//   - Maps each letter to its regional-indicator code point
//     (A → U+1F1E6, B → U+1F1E7, …, Z → U+1F1FF) and joins
//     the two as the flag string.
//
// Used by the timezone city-picker to prefix each entry with
// the country flag (mapping comes from
// [internal/domain/tools.LookupCountry] which parses zone1970.tab).
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

// TruncateShortcutLabel shortens a shortcut name to fit on a
// Telegram inline-keyboard button.
//
// Behavior:
//   - Returns name unchanged when its rune count is ≤ 40.
//   - Otherwise truncates to 39 runes and appends "…", giving
//     a 40-rune total. Rune-aware (uses [utf8.RuneCountInString])
//     so multi-byte characters don't get cut mid-codepoint.
//
// The full untruncated name is preserved separately in
// [callbacks.ShortMap]; this helper only affects display.
func TruncateShortcutLabel(name string) string {
	const maxLen = 40
	if utf8.RuneCountInString(name) <= maxLen {
		return name
	}
	runes := []rune(name)
	return string(runes[:maxLen-1]) + "…"
}

// filterIDArg substitutes the literal "-" for an empty filterID
// so callback positional args have a fixed-length layout. The
// handler's parser translates "-" back to "" before lookup.
//
// Without this trick, an unfiltered Prev/Next callback would
// have one fewer arg than a filtered one, and the parser would
// have to do positional-vs-named guesswork.
func filterIDArg(filterID string) string {
	if filterID == "" {
		return "-"
	}
	return filterID
}

// atLeastOne returns max(n, 1). Used in pagination headers so an
// empty list still renders as "Page 1/1" instead of "Page 1/0".
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// plural returns suffix when n != 1, "" when n == 1. Used to
// render English plurals in pagination headers ("1 match" /
// "2 matches", "1 timezone" / "2 timezones").
func plural(n int, suffix string) string {
	if n == 1 {
		return ""
	}
	return suffix
}

// ToolsDiskPanel renders the per-disk drill-down keyboard: one
// row of action buttons (Open in Finder always; Eject when
// removable=true), then Refresh + Back to Disks, then the
// standard Home row.
//
// Behavior:
//   - Open in Finder is always present and dispatches to
//     `tls:disk-open:<shortID>`.
//   - Eject is appended only when removable is true. The
//     handler trusts this gate — the keyboard is the boundary
//     that prevents an accidental "eject the root volume"
//     attempt.
//   - Refresh re-runs the per-disk drill-down via `tls:disk`;
//     Back returns to the disks list via `tls:disks`.
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
