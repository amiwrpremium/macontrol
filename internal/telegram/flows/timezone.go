package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewTimezone asks for a timezone string (e.g. "Europe/Istanbul") and sets it.
func NewTimezone(svc *tools.Service) Flow { return &timezoneFlow{svc: svc} }

type timezoneFlow struct{ svc *tools.Service }

func (timezoneFlow) Name() string { return "tls:tz" }

func (timezoneFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the timezone string (e.g. `Europe/Istanbul` or `UTC`). `/cancel` to abort."}
}

func (f *timezoneFlow) Handle(ctx context.Context, text string) Response {
	tz := strings.TrimSpace(text)
	if tz == "" {
		return Response{Text: "Empty. Send a timezone name."}
	}
	if err := f.svc.TimezoneSet(ctx, tz); err != nil {
		return Response{Text: fmt.Sprintf("⚠ set timezone failed: `%v`", err), Done: true}
	}
	cur, _ := f.svc.TimezoneCurrent(ctx)
	return Response{Text: fmt.Sprintf("✅ Timezone set — `%s`", cur), Done: true}
}
