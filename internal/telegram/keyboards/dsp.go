package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Display renders the 💡 Display dashboard.
func Display(state display.State) (text string, markup *models.InlineKeyboardMarkup) {
	if state.Level < 0 {
		text = "💡 *Display* — `unknown` (install `brew install brightness` for level readings)"
	} else {
		text = fmt.Sprintf("💡 *Display* — `%.0f%%`", state.Level*100)
	}
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "−10", CallbackData: callbacks.Encode(callbacks.NSDisplay, "down", "10")},
				{Text: "−5", CallbackData: callbacks.Encode(callbacks.NSDisplay, "down", "5")},
				{Text: "+5", CallbackData: callbacks.Encode(callbacks.NSDisplay, "up", "5")},
				{Text: "+10", CallbackData: callbacks.Encode(callbacks.NSDisplay, "up", "10")},
			},
			{
				{Text: "Set exact value…", CallbackData: callbacks.Encode(callbacks.NSDisplay, "set")},
			},
			{
				{Text: "🌙 Screensaver", CallbackData: callbacks.Encode(callbacks.NSDisplay, "screensaver")},
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSDisplay, "refresh")},
			},
			Nav(),
		},
	}
	return
}
