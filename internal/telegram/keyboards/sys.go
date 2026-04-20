package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// System renders the 🖥 System menu. Each button opens a specific read-only
// panel (the underlying messages themselves carry text + their own refresh).
func System() (text string, markup *models.InlineKeyboardMarkup) {
	text = "🖥 *System*"
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "ℹ Info", CallbackData: callbacks.Encode(callbacks.NSSystem, "info")},
				{Text: "🌡 Temperature", CallbackData: callbacks.Encode(callbacks.NSSystem, "temp")},
			},
			{
				{Text: "🧠 Memory", CallbackData: callbacks.Encode(callbacks.NSSystem, "mem")},
				{Text: "⚙ CPU", CallbackData: callbacks.Encode(callbacks.NSSystem, "cpu")},
			},
			{
				{Text: "📋 Top 10 processes", CallbackData: callbacks.Encode(callbacks.NSSystem, "top")},
				{Text: "🔪 Kill process…", CallbackData: callbacks.Encode(callbacks.NSSystem, "kill")},
			},
			Nav(),
		},
	}
	return
}

// SystemPanel builds a trailing refresh + nav for a sys panel (temp/mem/cpu).
func SystemPanel(action string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSystem, action)},
				{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSSystem, "open")},
			},
			Nav(),
		},
	}
}
