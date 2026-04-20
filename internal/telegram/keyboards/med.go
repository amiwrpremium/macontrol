package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Media renders the 📸 Media menu.
func Media() (text string, markup *models.InlineKeyboardMarkup) {
	text = "📸 *Media*\n\nScreenshots and screen recordings need *Screen Recording* TCC permission. Webcam photos need *Camera*."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📷 Screenshot", CallbackData: callbacks.Encode(callbacks.NSMedia, "shot")},
				{Text: "📷 Silent shot", CallbackData: callbacks.Encode(callbacks.NSMedia, "shot", "silent")},
			},
			{
				{Text: "📹 Record…", CallbackData: callbacks.Encode(callbacks.NSMedia, "record")},
				{Text: "📸 Webcam photo", CallbackData: callbacks.Encode(callbacks.NSMedia, "photo")},
			},
			Nav(),
		},
	}
	return
}
