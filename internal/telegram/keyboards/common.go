package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Nav returns the standard trailing row of `← Back` and `🏠 Home` buttons
// shown on every leaf dashboard.
func Nav() []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "🏠 Home", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// ConfirmRow returns a two-button row for destructive-action confirmation.
// action is the verb being confirmed (e.g. "shutdown"); ns is the namespace
// so the callback routes correctly.
func ConfirmRow(ns, action string) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "✅ Confirm", CallbackData: callbacks.Encode(ns, action, "ok")},
		{Text: "✖ Cancel", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// SingleRow turns a flat button list into a one-row keyboard.
func SingleRow(buttons ...models.InlineKeyboardButton) [][]models.InlineKeyboardButton {
	return [][]models.InlineKeyboardButton{buttons}
}
