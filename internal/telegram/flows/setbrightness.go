package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
)

// NewSetBrightness asks for a 0..100 percentage and sets display brightness.
func NewSetBrightness(svc *display.Service) Flow {
	return &setBrightnessFlow{svc: svc}
}

type setBrightnessFlow struct {
	svc *display.Service
}

func (setBrightnessFlow) Name() string { return "dsp:set" }

func (setBrightnessFlow) Start(_ context.Context) Response {
	return Response{Text: "Enter brightness `0`-`100`. Reply `/cancel` to abort."}
}

func (f *setBrightnessFlow) Handle(ctx context.Context, text string) Response {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || v < 0 || v > 100 {
		return Response{Text: "Please reply with a whole number between 0 and 100."}
	}
	st, err := f.svc.Set(ctx, float64(v)/100)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not set brightness: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("✅ Brightness set — `%.0f%%`", st.Level*100),
		Done: true,
	}
}
