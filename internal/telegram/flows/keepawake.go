package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/power"
)

// NewKeepAwake asks for minutes and starts `caffeinate -d -t`.
func NewKeepAwake(svc *power.Service) Flow {
	return &keepAwakeFlow{svc: svc}
}

type keepAwakeFlow struct{ svc *power.Service }

func (keepAwakeFlow) Name() string { return "pwr:keepawake" }

func (keepAwakeFlow) Start(_ context.Context) Response {
	return Response{Text: "Keep awake for how many minutes? (1-1440). Reply `/cancel` to abort."}
}

func (f *keepAwakeFlow) Handle(ctx context.Context, text string) Response {
	minutes, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || minutes < 1 || minutes > 1440 {
		return Response{Text: "Please reply with an integer between 1 and 1440."}
	}
	if err := f.svc.KeepAwake(ctx, time.Duration(minutes)*time.Minute); err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not start keep-awake: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("☕ Keep-awake running for %d min.", minutes),
		Done: true,
	}
}
