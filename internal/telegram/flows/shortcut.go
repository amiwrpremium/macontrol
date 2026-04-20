package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewShortcut asks for a Shortcut name and runs it.
func NewShortcut(svc *tools.Service) Flow { return &shortcutFlow{svc: svc} }

type shortcutFlow struct{ svc *tools.Service }

func (shortcutFlow) Name() string { return "tls:shortcut" }

func (shortcutFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the Shortcut name (case-sensitive). `/cancel` to abort."}
}

func (f *shortcutFlow) Handle(ctx context.Context, text string) Response {
	name := strings.TrimSpace(text)
	if name == "" {
		return Response{Text: "Empty. Send a Shortcut name."}
	}
	if err := f.svc.ShortcutRun(ctx, name); err != nil {
		return Response{Text: fmt.Sprintf("⚠ run failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Ran `%s`.", name), Done: true}
}
