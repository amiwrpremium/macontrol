package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleNotify(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Notify

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.Notify()
		return r.Edit(ctx, q, text, kb)

	case "send":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSendNotify(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "say":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSay(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown notify action.")
	return nil
}
