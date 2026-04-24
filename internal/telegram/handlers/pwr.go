package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handlePower is the Power dashboard's callback dispatcher.
// Reached via the [callbacks.NSPower] namespace from any tap on
// the ⚡ Power menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open"           → render the Power menu via
//     [keyboards.Power].
//  2. "lock"           → toast "Locking…", run
//     [power.Service.Lock]. No re-render — the screen is
//     about to lock; dashboard is moot.
//  3. "sleep"          → toast "Sleeping…", run
//     [power.Service.Sleep]. Same no-re-render rationale.
//  4. "restart" / "shutdown" / "logout" → confirm-then-execute
//     pattern shared across the three destructive actions:
//     a. First tap (no "ok" in args) → render the Confirm /
//     Cancel sub-keyboard via [keyboards.PowerConfirm]
//     with the action's label from [labelFor].
//     b. Second tap (with "ok" arg, dispatched by the
//     Confirm button) → toast "Executing <action>…",
//     run the matching [power.Service] method.
//  5. "keepawake"      → install [flows.NewKeepAwake] for a
//     typed duration in minutes.
//  6. "cancelawake"    → run [power.Service.CancelKeepAwake]
//     (`pkill -x caffeinate`); re-render the menu with a
//     "_keep-awake cancelled_" suffix.
//
// Unknown actions fall through to a "Unknown power action."
// toast.
//
// The destructive-action confirmation is a UI-layer guard
// only — a malicious caller emitting a callback with "ok"
// directly would bypass the confirm step. The whitelist
// (in bot.Deps.Whitelist) is the actual security boundary.
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

// labelFor maps a destructive-action callback name to its
// user-visible verb for the [keyboards.PowerConfirm] dialog.
//
// Behavior:
//   - Identity mapping for "restart" / "shutdown" / "logout"
//     (the three known destructive actions).
//   - Falls through to the input verbatim for unknown actions.
//     Used as a defensive fallback; the dispatcher above only
//     calls labelFor when data.Action is one of the three
//     known cases, so the fallback is theoretically
//     unreachable.
//
// The identity-mapping switch exists for future-proofing: if a
// new destructive action wants a different visible label
// ("powerOff" callback → "shut down" label), this is the place
// to remap.
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
