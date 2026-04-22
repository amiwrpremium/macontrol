package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleBattery(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Battery

	switch data.Action {
	case "open", "refresh":
		r.Ack(ctx, q)
		st, err := svc.Get(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔋 *Battery* — unavailable", err)
		}
		text, kb := keyboards.Battery(st)
		return r.Edit(ctx, q, text, kb)

	case "health":
		r.Ack(ctx, q)
		h, err := svc.GetHealth(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔋 *Battery health* — unavailable", err)
		}
		body := fmt.Sprintf("🔋 *Battery health*\n\n• Condition: `%s`\n• Cycle count: `%d`\n• Maximum capacity: `%s`\n• Adapter: `%s`",
			h.Condition, h.CycleCount, h.MaxCapacity, h.ChargerWattage)
		return r.Edit(ctx, q, body, keyboards.BatteryHealthPanel())
	}
	r.Toast(ctx, q, "Unknown battery action.")
	return nil
}
