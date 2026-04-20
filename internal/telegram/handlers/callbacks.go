package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// CallbackRouter implements bot.Router for inline-keyboard callbacks. It
// parses callback_data, looks up a namespace handler, and dispatches.
type CallbackRouter struct{}

// NewCallbackRouter returns a Router.
func NewCallbackRouter() *CallbackRouter { return &CallbackRouter{} }

// Handle implements bot.Router.
func (CallbackRouter) Handle(ctx context.Context, d *bot.Deps, update *models.Update) error {
	q := update.CallbackQuery
	if q == nil {
		return fmt.Errorf("callback handler received non-callback update")
	}
	data, err := callbacks.Decode(q.Data)
	if err != nil {
		Reply{Deps: d}.Toast(ctx, q, "Bad callback data.")
		return err
	}

	r := Reply{Deps: d}
	switch data.Namespace {
	case callbacks.NSNav:
		return handleNav(ctx, d, q, data)
	case callbacks.NSSound:
		return handleSound(ctx, d, q, data)
	case callbacks.NSDisplay:
		return handleDisplay(ctx, d, q, data)
	case callbacks.NSPower:
		return handlePower(ctx, d, q, data)
	case callbacks.NSBattery:
		return handleBattery(ctx, d, q, data)
	case callbacks.NSWifi:
		return handleWiFi(ctx, d, q, data)
	case callbacks.NSBT:
		return handleBluetooth(ctx, d, q, data)
	case callbacks.NSSystem:
		return handleSystem(ctx, d, q, data)
	case callbacks.NSMedia:
		return handleMedia(ctx, d, q, data)
	case callbacks.NSNotify:
		return handleNotify(ctx, d, q, data)
	case callbacks.NSTools:
		return handleTools(ctx, d, q, data)
	}
	r.Toast(ctx, q, "Unknown namespace.")
	return fmt.Errorf("unknown callback namespace: %q", data.Namespace)
}
