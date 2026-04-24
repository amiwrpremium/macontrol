package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Media renders the 📸 Media menu.
//
// Behavior:
//   - Static "📸 *Media*" header with an inline TCC reminder
//     so the user understands why a tap might silently fail
//     the first time. Screenshots and Record both need *Screen
//     Recording* permission; Webcam photo needs *Camera*. We
//     surface the requirement on the menu rather than only
//     after the failure because TCC denial returns no error
//     text the bot can forward.
//
// Keyboard rendering (3 rows):
//  1. Screenshot + Silent shot — both shell into screencapture;
//     "silent" passes the extra "silent" arg which the handler
//     translates to the `-x` flag (suppresses the camera
//     shutter sound).
//  2. Record… (installs [flows.NewRecord] for the typed-
//     duration flow) + Webcam photo (single-shot via
//     imagesnap).
//  3. Standard Back/Home nav row from [NavWithBack].
//
// The keyboard has no Refresh — Media is action-only with no
// state to re-fetch. Tapping any action returns to this same
// menu after the resulting photo/video is uploaded.
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
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}
