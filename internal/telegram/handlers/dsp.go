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

// handleDisplay is the Display dashboard's callback dispatcher.
// Reached via the [callbacks.NSDisplay] namespace from any tap
// on the 💡 Display menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → run [display.Service.Get], render
//     the dashboard via [keyboards.Display]. Get errors are
//     passed through to the keyboard renderer (NOT [errEdit])
//     so the dashboard shows the CoreDisplay-denial message
//     in-place — see PR #52 for the rationale.
//  2. "up" / "down"      → adjust brightness by ±delta/100
//     (delta defaults to 5%; data.Args[0] overrides). The
//     /100 conversion is because [display.State.Level] is
//     0.0..1.0 while the keyboard buttons think in
//     percent-delta integers.
//  3. "set"              → install [flows.NewSetBrightness]
//     for a typed exact value (0-100).
//  4. "screensaver"      → toast "Starting screen saver…"
//     then run [display.Service.Screensaver] which `open`s
//     ScreenSaverEngine.app. No re-render — the user is
//     about to see the screensaver, the dashboard is moot.
//
// Unlike most handlers, "open"/"refresh" does NOT route Get
// errors through [errEdit]. Reason: the brightness CLI's
// CoreDisplay-denied path returns a meaningful error that the
// dashboard renders in its own header line; surfacing it via
// errEdit would strip the keyboard and leave the user no path
// to retry / set / open screensaver.
//
// Unknown actions fall through to a "Unknown display action."
// toast.
// displayDispatch maps Display callback actions to handlers.
// "open"/"refresh" share a handler; "up"/"down" share one with
// a sign-flip inside.
var displayDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":        handleDisplayRefresh,
	"refresh":     handleDisplayRefresh,
	"up":          handleDisplayNudge,
	"down":        handleDisplayNudge,
	"set":         handleDisplaySet,
	"screensaver": handleDisplayScreensaver,
}

func handleDisplay(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := displayDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown display action.")
		return nil
	}
	return h(ctx, d, q, data)
}

func handleDisplayRefresh(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Display.Get(ctx)
	text, kb := keyboards.Display(st, err)
	return r.Edit(ctx, q, text, kb)
}

func handleDisplayNudge(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
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
	st, err := d.Services.Display.Adjust(ctx, f)
	if err != nil {
		return errEdit(ctx, r, q, "💡 *Display* — adjust failed", err)
	}
	text, kb := keyboards.Display(st, nil)
	return r.Edit(ctx, q, text, kb)
}

func handleDisplaySet(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	chatID := q.Message.Message.Chat.ID
	f := flows.NewSetBrightness(d.Services.Display)
	d.FlowReg.Install(chatID, f)
	return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
}

func handleDisplayScreensaver(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Toast(ctx, q, "Starting screen saver…")
	return d.Services.Display.Screensaver(ctx)
}
