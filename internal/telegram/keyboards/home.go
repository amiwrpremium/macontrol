package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Category is one tile in the home grid: a display label paired
// with the [callbacks] namespace whose handler the tile opens.
//
// Lifecycle:
//   - Defined statically in [Categories]; never constructed at
//     runtime. The slice is the single source of truth for both
//     the home-grid render and the dispatcher's startup
//     self-check ("every category has a handler").
//
// Field roles:
//   - Label is the user-visible button text, leading emoji
//     included.
//   - Namespace is one of the [callbacks] NS… constants. The
//     home button's callback_data is composed as
//     "<Namespace>:open" by [InlineHome].
type Category struct {
	// Label is the user-visible button text including its
	// leading emoji.
	Label string

	// Namespace is the [callbacks] NS… constant whose handler
	// this tile dispatches into.
	Namespace string
}

// Categories is the canonical, ordered list of every tile that
// appears on the home grid. The order here drives the inline
// keyboard layout in [InlineHome] (3 tiles per row, top-to-bottom,
// left-to-right).
//
// Adding a new dashboard category means: append a [Category] here,
// add the corresponding NS constant in [callbacks], register the
// handler in internal/telegram/handlers, and add a doc page in
// docs/usage/categories/. The dispatcher's self-check fails fast
// at boot if any of those steps are missed.
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

// InlineHome returns the inline home grid keyboard used by the
// /start, /menu, and "🏠 Home" responses across every dashboard.
//
// Behavior:
//   - Walks [Categories] in declaration order, building rows of
//     3 buttons each.
//   - Each button's callback_data is "<Namespace>:open" via
//     [callbacks.Encode], so tapping a tile dispatches into the
//     matching handler's "open" action.
//   - Any trailing partial row (1 or 2 buttons) is appended as
//     a final short row — currently 10 categories yields three
//     full rows + one extra button alone in a fourth row.
//
// Returns a fresh *[models.InlineKeyboardMarkup] each call;
// safe for callers to mutate the returned value (none do).
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

// HomeInlineTitle is the message text that pairs with [InlineHome]
// in every "go back to home" path: the /start and /menu command
// responses, and the "🏠 Home" button on every nested dashboard.
//
// The literal "🏠 *macontrol*" header doubles as the brand on
// first-message — a user opening the bot for the first time
// sees this exact string before any feature.
const HomeInlineTitle = "🏠 *macontrol*\n\nPick a category below."
