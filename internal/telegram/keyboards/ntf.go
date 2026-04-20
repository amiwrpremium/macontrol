package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Notify renders the 🔔 Notify menu.
func Notify() (text string, markup *models.InlineKeyboardMarkup) {
	text = "🔔 *Notify*\n\nPush a desktop notification to your Mac, or speak text through TTS."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✉ Send notification…", CallbackData: callbacks.Encode(callbacks.NSNotify, "send")},
			},
			{
				{Text: "🗣 Say…", CallbackData: callbacks.Encode(callbacks.NSNotify, "say")},
			},
			Nav(),
		},
	}
	return
}
