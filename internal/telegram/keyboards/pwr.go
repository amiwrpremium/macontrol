package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Power renders the ⚡ Power dashboard.
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
			Nav(),
		},
	}
	return
}

// PowerConfirm renders the confirmation sub-keyboard for a destructive
// action (e.g. shutdown, restart, logout).
func PowerConfirm(action, label string) (text string, markup *models.InlineKeyboardMarkup) {
	text = "⚠ *Confirm " + label + "*\n\nThis will affect your Mac immediately. Tap *Confirm* to proceed."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			ConfirmRow(callbacks.NSPower, action),
		},
	}
	return
}
