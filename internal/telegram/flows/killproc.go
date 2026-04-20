package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
)

// NewKillProc asks for a pid (integer) or a process name and kills it.
func NewKillProc(svc *system.Service) Flow {
	return &killProcFlow{svc: svc}
}

type killProcFlow struct{ svc *system.Service }

func (killProcFlow) Name() string { return "sys:kill" }

func (killProcFlow) Start(_ context.Context) Response {
	return Response{Text: "Send a PID (integer) or a process name to kill. Reply `/cancel` to abort."}
}

func (f *killProcFlow) Handle(ctx context.Context, text string) Response {
	text = strings.TrimSpace(text)
	if text == "" {
		return Response{Text: "Empty. Send a PID or name."}
	}
	if pid, err := strconv.Atoi(text); err == nil {
		if err := f.svc.Kill(ctx, pid); err != nil {
			return Response{Text: fmt.Sprintf("⚠ kill %d failed: `%v`", pid, err), Done: true}
		}
		return Response{Text: fmt.Sprintf("✅ SIGTERM sent to pid `%d`.", pid), Done: true}
	}
	if err := f.svc.KillByName(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ killall %s failed: `%v`", text, err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ `killall %s` done.", text), Done: true}
}
