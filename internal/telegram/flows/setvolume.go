package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
)

// NewSetVolume constructs a flow that asks for a 0..100 integer and sets
// the output volume.
func NewSetVolume(svc *sound.Service) Flow {
	return &setVolumeFlow{svc: svc}
}

type setVolumeFlow struct {
	svc *sound.Service
}

func (setVolumeFlow) Name() string { return "snd:set" }

func (setVolumeFlow) Start(_ context.Context) Response {
	return Response{Text: "Enter target volume (`0`-`100`). Reply `/cancel` to abort."}
}

func (f *setVolumeFlow) Handle(ctx context.Context, text string) Response {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || v < 0 || v > 100 {
		return Response{Text: "Please reply with a whole number between 0 and 100."}
	}
	st, err := f.svc.Set(ctx, v)
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not set volume: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("✅ Volume set — `%d%%` · muted: `%t`", st.Level, st.Muted),
		Done: true,
	}
}
