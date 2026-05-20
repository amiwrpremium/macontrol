package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Power renders the ⚡ Power dashboard.
//
// Behavior:
//   - Static header with the "Tap an action. Destructive
//     actions require a second tap to confirm." reminder so
//     users know Restart/Shutdown/Logout are gated.
//
// Keyboard rendering (4 rows):
//  1. Lock + Sleep — non-destructive, no confirm.
//  2. Restart + Shutdown + Logout — destructive trio that
//     route through [PowerConfirm] before executing. The
//     handler ([handlers.handlePower]) detects the missing
//     "ok" arg and renders the confirm dialog instead.
//  3. "☕ Keep awake…" + "🌑 Cancel awake" — caffeinate
//     install (typed-duration flow) and pkill respectively.
//  4. Standard Back/Home nav row.
//
// The destructive trio sit on a single row partly because
// they're conceptually grouped and partly because pairing
// them with Confirm-required gating means accidental
// taps cost only one extra tap to recover from.
func Power() (text string, markup *models.InlineKeyboardMarkup) {
	text = "⚡ *Power*\n\nTap an action. Destructive actions require a second tap to confirm."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔒 Lock", CallbackData: callbacks.Encode(callbacks.NSPower, "lock")},
				{Text: "💤 Sleep", CallbackData: callbacks.Encode(callbacks.NSPower, "sleep")},
			},
			{
				{Text: "🔁 Restart", CallbackData: callbacks.Encode(callbacks.NSPower, "restart")},
				{Text: "⏻ Shutdown", CallbackData: callbacks.Encode(callbacks.NSPower, "shutdown")},
				{Text: "🚪 Logout", CallbackData: callbacks.Encode(callbacks.NSPower, "logout")},
			},
			{
				{Text: "☕ Keep awake…", CallbackData: callbacks.Encode(callbacks.NSPower, "keepawake")},
				{Text: "🌑 Cancel awake", CallbackData: callbacks.Encode(callbacks.NSPower, "cancelawake")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}

// PowerConfirm renders the destructive-action confirmation
// sub-keyboard, used by Restart/Shutdown/Logout.
//
// Arguments:
//   - action is the original action name ("restart" /
//     "shutdown" / "logout") — re-used as the
//     `pwr:<action>:ok` callback that the handler matches
//     via [handlers.isConfirm].
//   - label is the user-visible verb shown in the header
//     ("Confirm restart"). Currently identical to action; see
//     [handlers.labelFor] for the future-proofing rationale.
//
// Behavior:
//   - Header: "⚠ *Confirm <label>*" + a one-line warning
//     about immediate effect.
//   - Single-row keyboard via [ConfirmRow]:
//     Confirm → `pwr:<action>:ok` (handler runs the action).
//     Cancel  → `pwr:open` (handler re-renders the Power
//     dashboard — NOT the home grid). The drill-back-to-
//     parent over jump-to-home distinction is intentional
//     and the reason ConfirmRow takes both NS args.
func PowerConfirm(action, label string) (text string, markup *models.InlineKeyboardMarkup) {
	text = "⚠ *Confirm " + label + "*\n\nThis will affect your Mac immediately. Tap *Confirm* to proceed."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			ConfirmRow(callbacks.NSPower, action, callbacks.NSPower, "open"),
		},
	}
	return
}
