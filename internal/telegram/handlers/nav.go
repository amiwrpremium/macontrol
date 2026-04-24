package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleNav is the cross-cutting navigation dispatcher.
// Reached via the [callbacks.NSNav] namespace from the always-
// present "🏠 Home" button (and any other future nav-only
// actions) on every nested screen.
//
// Routing rules (data.Action — first match wins):
//  1. "home" → edit the source message back to the inline
//     home grid via [keyboards.HomeInlineTitle] +
//     [keyboards.InlineHome]. Used as the universal "I'm
//     done with this drill-down, take me back to the top"
//     escape from any screen.
//
// Unknown actions fall through to a "Unknown nav action."
// toast.
//
// The Home button's callback is stamped onto every keyboard
// via [keyboards.Nav] / [keyboards.NavWithBack], so this
// dispatcher is hit from literally anywhere in the bot's UI.
// The handler intentionally has no other actions today — the
// "← Back" button on each screen routes into its OWNING
// namespace (e.g. `tls:open` to go back to Tools), not
// through nav.
func handleNav(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	if data.Action == "home" {
		r.Ack(ctx, q)
		return r.Edit(ctx, q, keyboards.HomeInlineTitle, keyboards.InlineHome())
	}
	r.Toast(ctx, q, "Unknown nav action.")
	return nil
}
