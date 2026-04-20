package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/domain/notify"
)

// NewSendNotify asks for "title | body" (or just a title, or just a body
// with a blank title) and delivers a desktop notification.
func NewSendNotify(svc *notify.Service) Flow {
	return &sendNotifyFlow{svc: svc}
}

type sendNotifyFlow struct{ svc *notify.Service }

func (sendNotifyFlow) Name() string { return "ntf:send" }

func (sendNotifyFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the notification as `title | body`, or just a body. `/cancel` to abort."}
}

func (f *sendNotifyFlow) Handle(ctx context.Context, text string) Response {
	title, body, _ := strings.Cut(text, "|")
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if body == "" {
		body = title
		title = ""
	}
	if body == "" {
		return Response{Text: "Empty notification. Try again or `/cancel`."}
	}
	transport, err := f.svc.Notify(ctx, notify.Opts{Title: title, Body: body})
	if err != nil {
		return Response{Text: fmt.Sprintf("⚠ notify failed: `%v`", err), Done: true}
	}
	return Response{Text: fmt.Sprintf("✅ Notified via `%s`.", transport), Done: true}
}

// NewSay asks for text and speaks it via `say`.
func NewSay(svc *notify.Service) Flow { return &sayFlow{svc: svc} }

type sayFlow struct{ svc *notify.Service }

func (sayFlow) Name() string { return "ntf:say" }

func (sayFlow) Start(_ context.Context) Response {
	return Response{Text: "Send the text to speak. `/cancel` to abort."}
}

func (f *sayFlow) Handle(ctx context.Context, text string) Response {
	if strings.TrimSpace(text) == "" {
		return Response{Text: "Empty. Send the text or `/cancel`."}
	}
	if err := f.svc.Say(ctx, text); err != nil {
		return Response{Text: fmt.Sprintf("⚠ say failed: `%v`", err), Done: true}
	}
	return Response{Text: "✅ Spoken.", Done: true}
}
