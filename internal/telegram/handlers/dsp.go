package handlers

import (
	"context"
	"strconv"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleDisplay(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Display

	switch data.Action {
	case "open", "refresh":
		r.Ack(ctx, q)
		st, err := svc.Get(ctx)
		text, kb := keyboards.Display(st, err)
		return r.Edit(ctx, q, text, kb)

	case "up", "down":
		delta := 5
		if len(data.Args) > 0 {
			if v, err := strconv.Atoi(data.Args[0]); err == nil {
				delta = v
			}
		}
		f := float64(delta) / 100
		if data.Action == "down" {
			f = -f
		}
		r.Ack(ctx, q)
		st, err := svc.Adjust(ctx, f)
		if err != nil {
			return errEdit(ctx, r, q, "💡 *Display* — adjust failed", err)
		}
		text, kb := keyboards.Display(st, nil)
		return r.Edit(ctx, q, text, kb)

	case "set":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSetBrightness(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "screensaver":
		r.Toast(ctx, q, "Starting screen saver…")
		return svc.Screensaver(ctx)
	}
	r.Toast(ctx, q, "Unknown display action.")
	return nil
}
