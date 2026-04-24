package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleBattery is the Battery dashboard's callback dispatcher.
// Reached via the [callbacks.NSBattery] namespace from any tap
// on the 🔋 Battery menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → run [battery.Service.Get], render
//     the dashboard via [keyboards.Battery]. Both share the
//     same code path.
//  2. "health"           → run [battery.Service.GetHealth],
//     render the labelled health-drill-down panel via
//     [keyboards.BatteryHealthPanel]. Slower (~1s; uses
//     `system_profiler SPPowerDataType`) so it's gated behind
//     an explicit drill-down rather than included on the main
//     panel.
//
// Unknown actions fall through to a "Unknown battery action."
// toast.
//
// On Macs without a battery (Mac mini, Mac Studio, Mac Pro),
// the underlying [battery.Service.Get] returns
// Status{Present: false, Percent: -1} and [keyboards.Battery]
// renders the "not present (desktop Mac)" panel; no special
// case is needed in this dispatcher.
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
