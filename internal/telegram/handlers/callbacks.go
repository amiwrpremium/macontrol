package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// CallbackRouter dispatches inline-keyboard callback_query
// updates to the matching per-namespace handler. Implements
// [bot.Router] so the upstream dispatcher can install it on
// bot.Deps.Calls.
//
// Lifecycle:
//   - Constructed once at daemon startup via [NewCallbackRouter]
//     and stored on bot.Deps.Calls.
//
// Concurrency:
//   - Stateless; every Handle call is independent. The
//     per-namespace handlers themselves are stateless too — all
//     state lives on bot.Deps.
type CallbackRouter struct{}

// NewCallbackRouter returns a fresh [CallbackRouter] ready to
// be installed on bot.Deps.Calls.
func NewCallbackRouter() *CallbackRouter { return &CallbackRouter{} }

// Handle is the [bot.Router] implementation for callback queries.
//
// Behavior:
//  1. Validates that update.CallbackQuery is non-nil. Returns
//     a sentinel error otherwise (defensive — the dispatcher
//     in bot.go should never route a non-callback update here,
//     but a future refactor could).
//  2. Parses callback_data via [callbacks.Decode]. On parse
//     failure, toasts "Bad callback data." and returns the
//     decode error so the dispatcher logs it.
//  3. Switches on [callbacks.Data.Namespace] to dispatch to the
//     matching per-namespace handler. The 11 cases mirror the
//     NS… constants in the [callbacks] package and the
//     handler files in this directory (snd.go, dsp.go, pwr.go,
//     wif.go, bt.go, bat.go, sys.go, med.go, ntf.go, tls.go,
//     nav.go).
//  4. Unknown namespace → toast "Unknown namespace." and
//     return an error. Should never happen in practice — every
//     keyboard rendered by the package emits one of the known
//     NS values, and [callbacks.AllNamespaces] is what the
//     dispatcher checks against at startup.
//
// Errors returned here propagate up to [bot.Deps.dispatch] and
// are logged at WARN. The user's original tap has already been
// answered (toast, ack, or edit) by the time we return — the
// error is for diagnostics only.
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
