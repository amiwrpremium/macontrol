package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Tools renders the 🛠 Tools menu. features gates the Shortcuts runner
// (needs macOS 13+).
func Tools(features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	text = "🛠 *Tools*"
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "📋 Clipboard (read)", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "get")},
			{Text: "📋 Clipboard (set)…", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "set")},
		},
		{
			{Text: "🧭 Timezone…", CallbackData: callbacks.Encode(callbacks.NSTools, "tz")},
			{Text: "🔄 Sync time", CallbackData: callbacks.Encode(callbacks.NSTools, "synctime")},
		},
		{
			{Text: "💿 Disks", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
		},
	}
	if features.Shortcuts {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "⚡ Run Shortcut…", CallbackData: callbacks.Encode(callbacks.NSTools, "shortcut")},
		})
	}
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}
