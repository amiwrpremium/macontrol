// Package keyboards builds Telegram reply and inline keyboards. Pure UI —
// no side effects, no domain imports. Keeps layout unit-testable.
package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Category bundles a label + inline-keyboard namespace for the home grid.
type Category struct {
	Label     string
	Namespace string
}

// Categories is the canonical home list. Order here drives both the
// ReplyKeyboard and the inline home grid.
var Categories = []Category{
	{"🔊 Sound", callbacks.NSSound},
	{"💡 Display", callbacks.NSDisplay},
	{"🔋 Battery", callbacks.NSBattery},
	{"📶 Wi-Fi", callbacks.NSWifi},
	{"🔵 Bluetooth", callbacks.NSBT},
	{"⚡ Power", callbacks.NSPower},
	{"🖥 System", callbacks.NSSystem},
	{"📸 Media", callbacks.NSMedia},
	{"🔔 Notify", callbacks.NSNotify},
	{"🛠 Tools", callbacks.NSTools},
}

// ReplyHome returns the one-shot ReplyKeyboard with 3 buttons per row.
func ReplyHome() *models.ReplyKeyboardMarkup {
	rows := make([][]models.KeyboardButton, 0, (len(Categories)+2)/3+1)
	row := make([]models.KeyboardButton, 0, 3)
	for _, c := range Categories {
		row = append(row, models.KeyboardButton{Text: c.Label})
		if len(row) == 3 {
			rows = append(rows, row)
			row = make([]models.KeyboardButton, 0, 3)
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	// Final row of utility buttons.
	rows = append(rows, []models.KeyboardButton{
		{Text: "❓ Help"},
		{Text: "❌ Cancel"},
	})
	return &models.ReplyKeyboardMarkup{
		Keyboard:        rows,
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		IsPersistent:    false,
	}
}

// InlineHome returns the inline home grid used when returning from a leaf.
// Each button opens the category's dashboard via a `<ns>:open` callback.
func InlineHome() *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, (len(Categories)+2)/3)
	row := make([]models.InlineKeyboardButton, 0, 3)
	for _, c := range Categories {
		row = append(row, models.InlineKeyboardButton{
			Text:         c.Label,
			CallbackData: callbacks.Encode(c.Namespace, "open"),
		})
		if len(row) == 3 {
			rows = append(rows, row)
			row = make([]models.InlineKeyboardButton, 0, 3)
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// HomeWelcome is the text shown with ReplyHome on /start or /menu.
const HomeWelcome = "🏠 *macontrol*\n\nPick a category below, or tap an inline button to dive into a dashboard."

// HomeInlineTitle is the text for the inline home grid (shown when editing
// a leaf message back to home).
const HomeInlineTitle = "🏠 *Home*\n\nPick a category."

// CategoryByLabel returns the callback namespace associated with a
// ReplyKeyboard label (e.g. "🔊 Sound" → callbacks.NSSound). Lets the
// dispatcher route reply-keyboard taps to the same handlers as their
// inline `:open` callbacks. Returns ("", false) for unknown labels.
func CategoryByLabel(label string) (string, bool) {
	for _, c := range Categories {
		if c.Label == label {
			return c.Namespace, true
		}
	}
	return "", false
}
