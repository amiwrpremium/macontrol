package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Display renders the 💡 Display dashboard. Unique among the
// per-category keyboards in that it accepts the underlying
// read error directly — see PR #52 + the [display] package
// doc for why "CoreDisplay denied" needs to be rendered
// in-place rather than stripped via [handlers.errEdit].
//
// Header rendering (first match wins):
//  1. err != nil → "💡 *Display* — `level unknown`\n⚠ `<err>`"
//     so the user sees both the unknown-level state AND the
//     CLI's own diagnostic text.
//  2. state.Level < 0 (sentinel for "unknown" without a
//     captured error — typical when the brightness CLI
//     isn't installed at all) → "💡 *Display* — `level
//     unknown`" with no diagnostic line.
//  3. Otherwise → "💡 *Display* — `<pct>%`" where pct is
//     state.Level * 100, rounded to integer.
//
// Keyboard rendering (4 rows):
//  1. ±10 / ±5 brightness nudges. The keyboard speaks
//     percent; the handler converts to fraction before
//     passing to [display.Service.Adjust]. No ±1 row —
//     brightness has fewer perceptible steps than volume.
//  2. "Set exact value…" → installs [flows.NewSetBrightness].
//  3. "🌙 Screensaver" + "🔄 Refresh".
//  4. Standard Back/Home nav row.
//
// The buttons are present even when Level is unknown so the
// user can blind-set / blind-screensaver as a recovery path.
func Display(state display.State, err error) (text string, markup *models.InlineKeyboardMarkup) {
	switch {
	case err != nil:
		text = fmt.Sprintf("💡 *Display* — `level unknown`\n⚠ `%v`", err)
	case state.Level < 0:
		text = "💡 *Display* — `level unknown`"
	default:
		text = fmt.Sprintf("💡 *Display* — `%.0f%%`", state.Level*100)
	}
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "−10", CallbackData: callbacks.Encode(callbacks.NSDisplay, "down", "10")},
				{Text: "−5", CallbackData: callbacks.Encode(callbacks.NSDisplay, "down", "5")},
				{Text: "+5", CallbackData: callbacks.Encode(callbacks.NSDisplay, "up", "5")},
				{Text: "+10", CallbackData: callbacks.Encode(callbacks.NSDisplay, "up", "10")},
			},
			{
				{Text: "Set exact value…", CallbackData: callbacks.Encode(callbacks.NSDisplay, "set")},
			},
			{
				{Text: "🌙 Screensaver", CallbackData: callbacks.Encode(callbacks.NSDisplay, "screensaver")},
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSDisplay, "refresh")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}
