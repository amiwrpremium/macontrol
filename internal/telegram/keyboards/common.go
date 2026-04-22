package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Nav returns the trailing row that holds only the always-on Home
// button. Use it on screens that have no parent (the home grid
// itself).
func Nav() []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "🏠 Home", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// NavWithBack returns the standard trailing row for any nested
// screen: a `← Back` button pointing at the immediate parent, and
// the always-on `🏠 Home`. Even when Back's destination is the home
// grid (level-1 menus), keeping the explicit affordance makes the
// navigation pattern uniform across every depth.
func NavWithBack(backNS, backAction string, backArgs ...string) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "← Back", CallbackData: callbacks.Encode(backNS, backAction, backArgs...)},
		{Text: "🏠 Home", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// ConfirmRow returns a two-button row for destructive-action
// confirmation. confirmNS/confirmAction is invoked (with the "ok"
// argument) when the user taps Confirm. cancelNS/cancelAction is
// where Cancel navigates — typically the parent dashboard, NOT Home.
func ConfirmRow(confirmNS, confirmAction, cancelNS, cancelAction string) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "✅ Confirm", CallbackData: callbacks.Encode(confirmNS, confirmAction, "ok")},
		{Text: "✖ Cancel", CallbackData: callbacks.Encode(cancelNS, cancelAction)},
	}
}

// SingleRow turns a flat button list into a one-row keyboard.
func SingleRow(buttons ...models.InlineKeyboardButton) [][]models.InlineKeyboardButton {
	return [][]models.InlineKeyboardButton{buttons}
}
