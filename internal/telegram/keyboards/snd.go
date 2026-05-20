package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Sound renders the 🔊 Sound dashboard for the supplied
// [sound.State] snapshot.
//
// Behavior:
//
// Header rendering:
//   - "🔊 *Sound* — `<level>%` · <unmuted|MUTED>". The MUTED
//     label is uppercase to draw attention; "unmuted" is the
//     normal-state label.
//
// Keyboard rendering (4 rows):
//  1. Volume nudges + dynamic mute button: −5 / −1 /
//     [Mute|Unmute] / +1 / +5. The middle button switches
//     between "🔇 Mute" (when unmuted, callback `snd:mute`)
//     and "🔈 Unmute" (when muted, callback `snd:unmute`).
//  2. "Set exact value…" — installs the
//     [flows.NewSetVolume] typed-value flow.
//  3. "🔊 MAX (100)" + "🔄 Refresh".
//  4. Standard Back/Home nav row via [NavWithBack].
//
// The asymmetric ±5 / ±1 deltas (no ±10) match macOS's own
// volume-key behaviour: long-press for ±5, single-tap for ±1.
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
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}
