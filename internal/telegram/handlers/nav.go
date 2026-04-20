package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleNav(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	switch data.Action {
	case "home":
		r.Ack(ctx, q)
		return r.Edit(ctx, q, keyboards.HomeInlineTitle, keyboards.InlineHome())
	}
	r.Toast(ctx, q, "Unknown nav action.")
	return nil
}
