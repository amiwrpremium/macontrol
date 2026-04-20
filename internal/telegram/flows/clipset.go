package flows

import (
	"context"
	"fmt"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
)

// NewClipSet asks for text and writes it to the clipboard.
func NewClipSet(svc *tools.Service) Flow { return &clipSetFlow{svc: svc} }

type clipSetFlow struct{ svc *tools.Service }

func (clipSetFlow) Name() string { return "tls:clipset" }

func (clipSetFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the text you want on the clipboard. `/cancel` to abort."}
}

func (f *clipSetFlow) Handle(ctx context.Context, text string) Response {
	if err := f.svc.ClipboardWrite(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ clipboard write failed: `%v`", err), Done: true}
	}
	return Response{Text: "✅ Clipboard updated.", Done: true}
}
