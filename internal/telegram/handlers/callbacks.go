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

// namespaceDispatch maps each callback namespace to its per-
// namespace entry handler. One entry per keyboard category.
var namespaceDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	callbacks.NSNav:     handleNav,
	callbacks.NSSound:   handleSound,
	callbacks.NSDisplay: handleDisplay,
	callbacks.NSPower:   handlePower,
	callbacks.NSBattery: handleBattery,
	callbacks.NSWifi:    handleWiFi,
	callbacks.NSBT:      handleBluetooth,
	callbacks.NSSystem:  handleSystem,
	callbacks.NSMedia:   handleMedia,
	callbacks.NSNotify:  handleNotify,
	callbacks.NSTools:   handleTools,
}

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
//  3. Looks up [namespaceDispatch] for the per-namespace entry
//     handler. The 11 entries mirror the NS… constants in the
//     [callbacks] package and the handler files in this
//     directory (snd.go, dsp.go, pwr.go, wif.go, bt.go, bat.go,
//     sys.go, med.go, ntf.go, tls.go, nav.go).
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
	h, ok := namespaceDispatch[data.Namespace]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown namespace.")
		return fmt.Errorf("unknown callback namespace: %q", data.Namespace)
	}
	return h(ctx, d, q, data)
}
