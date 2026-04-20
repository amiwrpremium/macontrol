package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Sound renders the 🔊 Sound dashboard for the given current state.
func Sound(state sound.State) (text string, markup *models.InlineKeyboardMarkup) {
	muted := "unmuted"
	if state.Muted {
		muted = "MUTED"
	}
	text = fmt.Sprintf("🔊 *Sound* — `%d%%` · %s", state.Level, muted)

	muteBtn := models.InlineKeyboardButton{
		Text: "🔇 Mute", CallbackData: callbacks.Encode(callbacks.NSSound, "mute"),
	}
	if state.Muted {
		muteBtn = models.InlineKeyboardButton{
			Text: "🔈 Unmute", CallbackData: callbacks.Encode(callbacks.NSSound, "unmute"),
		}
	}

	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "−5", CallbackData: callbacks.Encode(callbacks.NSSound, "down", "5")},
				{Text: "−1", CallbackData: callbacks.Encode(callbacks.NSSound, "down", "1")},
				muteBtn,
				{Text: "+1", CallbackData: callbacks.Encode(callbacks.NSSound, "up", "1")},
				{Text: "+5", CallbackData: callbacks.Encode(callbacks.NSSound, "up", "5")},
			},
			{
				{Text: "Set exact value…", CallbackData: callbacks.Encode(callbacks.NSSound, "set")},
			},
			{
				{Text: "🔊 MAX (100)", CallbackData: callbacks.Encode(callbacks.NSSound, "max")},
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSound, "refresh")},
			},
			Nav(),
		},
	}
	return
}
