package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// NewTimezoneSearch returns a one-step flow that asks the user for
// a substring, filters the timezones in `region`, and renders the
// filtered city list as the flow's terminal response. The substring
// is stashed in the ShortMap so subsequent Prev/Next pagination
// preserves the filter via callback args alone.
func NewTimezoneSearch(svc *tools.Service, sm *callbacks.ShortMap, region string) Flow {
	return &timezoneSearchFlow{svc: svc, sm: sm, region: region}
}

type timezoneSearchFlow struct {
	svc    *tools.Service
	sm     *callbacks.ShortMap
	region string
}

func (timezoneSearchFlow) Name() string { return "tls:tz-search" }

func (f timezoneSearchFlow) Start(_ context.Context) Response {
	return Response{
		Text: fmt.Sprintf("🔍 Send a substring (case-insensitive) to filter `%s` timezones. `/cancel` to abort.", f.region),
	}
}

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

// FilterTimezonesInRegion returns timezones in `region` whose name
// contains sub (case-insensitive). Empty sub returns every timezone
// in the region. Exported so the handler's tz-page case can reuse
// the same matcher.
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

// PageTimezones slices the per-region list to the requested page,
// builds button labels (flag + city path), and registers fresh
// ShortMap ids. Returns the page items + total page count (≥1).
// Lives in flows so handlers and the search flow share one
// implementation.
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
