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

// Categories is the canonical home list. Order here drives the inline
// home grid layout.
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

// HomeInlineTitle is the text for the inline home grid, used both as
// the response to /start and /menu and when editing a leaf message
// back to home.
const HomeInlineTitle = "🏠 *macontrol*\n\nPick a category below."
