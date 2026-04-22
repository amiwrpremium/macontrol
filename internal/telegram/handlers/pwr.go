package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handlePower(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Power

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.Power()
		return r.Edit(ctx, q, text, kb)

	case "lock":
		r.Toast(ctx, q, "Locking…")
		return svc.Lock(ctx)

	case "sleep":
		r.Toast(ctx, q, "Sleeping…")
		return svc.Sleep(ctx)

	case "restart", "shutdown", "logout":
		if isConfirm(data.Args) {
			r.Toast(ctx, q, "Executing "+data.Action+"…")
			switch data.Action {
			case "restart":
				return svc.Restart(ctx)
			case "shutdown":
				return svc.Shutdown(ctx)
			case "logout":
				return svc.Logout(ctx)
			}
		}
		r.Ack(ctx, q)
		label := labelFor(data.Action)
		text, kb := keyboards.PowerConfirm(data.Action, label)
		return r.Edit(ctx, q, text, kb)

	case "keepawake":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewKeepAwake(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "cancelawake":
		r.Toast(ctx, q, "Cancelling keep-awake…")
		if err := svc.CancelKeepAwake(ctx); err != nil {
			return errEdit(ctx, r, q, "⚡ *Power* — cancel failed", err)
		}
		text, kb := keyboards.Power()
		return r.Edit(ctx, q, text+"\n\n_keep-awake cancelled_", kb)
	}
	r.Toast(ctx, q, "Unknown power action.")
	return nil
}

func labelFor(action string) string {
	switch action {
	case "restart":
		return "restart"
	case "shutdown":
		return "shutdown"
	case "logout":
		return "logout"
	}
	return action
}
