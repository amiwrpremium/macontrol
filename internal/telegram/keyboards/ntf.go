package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Notify renders the 🔔 Notify menu.
//
// Behavior:
//   - Static "🔔 *Notify*" header with a one-line tagline
//     describing the two outputs: desktop notification banner
//     vs. spoken text via macOS's `say` CLI. The tagline is
//     informational — there's no per-item state to surface.
//
// Keyboard rendering (3 rows):
//  1. Send notification… — installs the [flows.NewNotify]
//     typed-message flow. The trailing ellipsis follows the
//     project convention for "this opens a typed-input
//     prompt".
//  2. Say… — installs [flows.NewSay] with the same typed-
//     message convention. Separate row from "Send" because
//     they're different output modalities, not variations of
//     the same action.
//  3. Standard Back/Home nav row from [NavWithBack].
//
// No Refresh — Notify is action-only with no state to
// re-fetch. Both actions return to this same menu after the
// notification is delivered (or the speech finishes).
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
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}
