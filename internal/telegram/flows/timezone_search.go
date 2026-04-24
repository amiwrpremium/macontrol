package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// NewTimezoneSearch returns the one-step typed-input [Flow]
// that filters the city list inside a single timezone region
// (e.g. all "Europe/*" cities) by a user-supplied substring,
// then renders the filtered cities page as the flow's
// terminal response.
//
// Behavior:
//   - Belongs to the timezone picker subsystem; reached when
//     the user taps "🔍 Search" on the city-list keyboard.
//   - The substring is stashed in the [callbacks.ShortMap] so
//     subsequent Prev/Next pagination keeps the filter via
//     callback args alone (no flow-instance state survives
//     the terminal Done=true response).
//   - One-shot: terminates after rendering page 0 of the
//     filtered results — Prev/Next from there go through the
//     handler's tz-page case, not back through this flow.
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleTools] when the user taps "🔍 Search" on
// [keyboards.ToolsTimezoneCities].
func NewTimezoneSearch(svc *tools.Service, sm *callbacks.ShortMap, region string) Flow {
	return &timezoneSearchFlow{svc: svc, sm: sm, region: region}
}

// timezoneSearchFlow is the [NewTimezoneSearch]-returned
// [Flow]. Holds the service, the shared ShortMap (so the
// filter substring + per-row tz strings round-trip through
// callback data), and the region scope.
//
// Field roles:
//   - svc is the [tools.Service] used by Handle to enumerate
//     and look up the current timezone.
//   - sm is the shared [callbacks.ShortMap] — the same one
//     the bot passes to every handler, so ShortID values
//     emitted by Handle are valid for subsequent
//     Prev/Next/select callbacks.
//   - region is the IANA region prefix (e.g. "Europe") set
//     when the flow was constructed; never changes during the
//     flow's lifetime.
type timezoneSearchFlow struct {
	svc    *tools.Service
	sm     *callbacks.ShortMap
	region string
}

// Name returns the dispatcher log identifier "tls:tz-search".
func (timezoneSearchFlow) Name() string { return "tls:tz-search" }

// Start emits the typed-input prompt mentioning the region
// the search is scoped to.
func (f timezoneSearchFlow) Start(_ context.Context) Response {
	return Response{
		Text: fmt.Sprintf("🔍 Send a substring (case-insensitive) to filter `%s` timezones. `/cancel` to abort.", f.region),
	}
}

// Handle filters the region's timezones by the user
// substring and renders the page-0 results.
//
// Routing rules (first match wins):
//  1. Trimmed text empty → "Empty filter — send some text or
//     `/cancel`." (NOT terminal — re-prompted).
//  2. [tools.Service.TimezoneList] returns non-nil err → "⚠
//     couldn't list timezones: `<err>`" + Done.
//  3. Otherwise:
//     - Filter via [FilterTimezonesInRegion].
//     - Read current tz via [tools.Service.TimezoneCurrent]
//     (its error is silently ignored — current is empty on
//     failure, which the keyboard renders as "no current
//     timezone" gracefully).
//     - Stash the substring in ShortMap → filterID for use in
//     pagination callbacks.
//     - Render via [keyboards.ToolsTimezoneCities] and emit
//     the resulting (text, markup) + Done=true.
func (f *timezoneSearchFlow) Handle(ctx context.Context, text string) Response {
	sub := strings.TrimSpace(text)
	if sub == "" {
		return Response{Text: "Empty filter — send some text or `/cancel`."}
	}
	all, err := f.svc.TimezoneList(ctx)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ couldn't list timezones: `%v`", err), Done: true}
	}
	matches := FilterTimezonesInRegion(all, f.region, sub)
	current, _ := f.svc.TimezoneCurrent(ctx)
	filterID := f.sm.Put(sub)
	items, totalPages := PageTimezones(matches, f.region, 0, f.sm)
	headerText, kb := keyboards.ToolsTimezoneCities(f.region, current, items, 0, totalPages, len(matches), filterID, sub)
	return Response{
		Text:   headerText,
		Markup: kb,
		Done:   true,
	}
}

// FilterTimezonesInRegion returns the subset of `all`
// timezones whose name starts with "<region>/" AND
// (case-insensitively) contains sub.
//
// Behavior:
//   - Empty sub returns every timezone in the region (no
//     contains check).
//   - Match is case-insensitive on both sides via
//     [strings.ToLower].
//   - Region prefix check is case-sensitive and includes the
//     trailing slash, so "Europe" does NOT match "Europe/Berlin"
//     callers passing region="Europe" — the prefix is built
//     internally as "Europe/".
//
// Exported so the handler's tz-page case can reuse the same
// matcher when the user paginates through filtered results
// (the filter substring is recovered from ShortMap and re-
// applied each page render).
func FilterTimezonesInRegion(all []string, region, sub string) []string {
	prefix := region + "/"
	needle := strings.ToLower(sub)
	out := make([]string, 0, 64)
	for _, tz := range all {
		if !strings.HasPrefix(tz, prefix) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(tz), needle) {
			continue
		}
		out = append(out, tz)
	}
	return out
}

// PageTimezones slices the per-region city list to the
// requested page, builds button labels (flag + city path), and
// registers a fresh [callbacks.ShortMap] id per row. Lives in
// flows (not keyboards) so handlers and the search flow share
// one implementation.
//
// Behavior:
//   - Page size is [keyboards.TimezonesPageSize] (currently
//     15).
//   - Returns at least 1 totalPages even when cities is empty,
//     so the keyboard always has something to render.
//   - Negative or out-of-range page values are clamped:
//     page < 0 → 0; page >= totalPages → totalPages-1.
//   - Empty slice case: start == end, items is the empty slice
//     (no panic).
//   - Per-row label: "🇩🇪 Berlin" when the country lookup
//     succeeds, plain "Berlin" otherwise. Truncated to fit the
//     button via [keyboards.TruncateShortcutLabel] (yes, the
//     shortcut helper is reused — see the smells list).
//   - Per-row callback uses sm.Put(tz) so the full IANA name
//     can be recovered when the user taps the row, regardless
//     of length (timezones can blow the 64-byte callback_data
//     budget alone, e.g. "America/Argentina/Buenos_Aires").
//
// Returns the page slice and total page count.
func PageTimezones(cities []string, region string, page int, sm *callbacks.ShortMap) ([]keyboards.TimezoneListItem, int) {
	total := len(cities)
	totalPages := (total + keyboards.TimezonesPageSize - 1) / keyboards.TimezonesPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * keyboards.TimezonesPageSize
	end := start + keyboards.TimezonesPageSize
	if end > total {
		end = total
	}
	if start > end {
		start = end
	}
	prefix := region + "/"
	items := make([]keyboards.TimezoneListItem, 0, end-start)
	for _, tz := range cities[start:end] {
		city := strings.TrimPrefix(tz, prefix)
		label := city
		if iso, ok := tools.LookupCountry(tz); ok {
			if flag := keyboards.FlagFromISO2(iso); flag != "" {
				label = flag + " " + city
			}
		}
		items = append(items, keyboards.TimezoneListItem{
			Label:   keyboards.TruncateShortcutLabel(label),
			ShortID: sm.Put(tz),
		})
	}
	return items, totalPages
}
