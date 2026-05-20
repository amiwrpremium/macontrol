package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Nav returns the trailing keyboard row that holds only the
// always-on "🏠 Home" button. Used on screens that have no
// parent — currently just the home grid itself, where Back
// would be a no-op.
//
// Behavior:
//   - Single button row.
//   - Callback shape: "nav:home" via [callbacks.Encode] with
//     the [callbacks.NSNav] namespace and "home" action,
//     dispatched by [handlers.handleNav].
func Nav() []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "🏠 Home", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// NavWithBack returns the standard trailing row for any
// nested screen: a "← Back" button pointing at the immediate
// parent, and the always-on "🏠 Home".
//
// Behavior:
//   - Two-button row in (Back, Home) order. Back is on the
//     left so it falls under the user's thumb on a phone.
//   - Back's callback is encoded from (backNS, backAction,
//     backArgs…) — typically the parent dashboard's "open"
//     action, e.g. NavWithBack(callbacks.NSPower, "open")
//     for a Power drill-down's Back-to-Power button.
//   - Home is the same "nav:home" as [Nav].
//
// Even on a one-level-deep menu (where Back's destination
// equals Home's), keeping the explicit affordance makes the
// navigation pattern uniform across every depth — see
// docs/getting-started/first-message.md.
func NavWithBack(backNS, backAction string, backArgs ...string) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "← Back", CallbackData: callbacks.Encode(backNS, backAction, backArgs...)},
		{Text: "🏠 Home", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	}
}

// ConfirmRow returns the two-button confirmation row used by
// destructive-action dashboards (Power's restart/shutdown/
// logout, System's force-kill).
//
// Behavior:
//   - Confirm button stamps the "ok" sentinel as the first
//     callback arg — see [handlers.isConfirm] which checks
//     for this exact literal to gate the destructive
//     execution.
//   - Cancel routes to (cancelNS, cancelAction) — typically
//     the parent dashboard's "open", NOT [Nav]'s "home".
//     The distinction matters: a Cancel on a Power confirm
//     should return to the Power menu, not bounce all the
//     way back to the home grid.
//
// The cancelNS argument lets a destructive action live in
// one namespace while the parent dashboard lives in another
// (rare but supported, e.g. cross-namespace confirms).
func ConfirmRow(confirmNS, confirmAction, cancelNS, cancelAction string) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: "✅ Confirm", CallbackData: callbacks.Encode(confirmNS, confirmAction, "ok")},
		{Text: "✖ Cancel", CallbackData: callbacks.Encode(cancelNS, cancelAction)},
	}
}

// SingleRow wraps a flat slice of buttons into a one-row
// keyboard structure (`[][]models.InlineKeyboardButton`).
// Convenience for the cases where building the outer slice
// inline is clutter — e.g. a single-row dashboard helper that
// hands the result straight to a handler.
//
// Used sparingly across the package; most builders compose
// rows manually with `[][]Button{{...}, {...}}` literals.
func SingleRow(buttons ...models.InlineKeyboardButton) [][]models.InlineKeyboardButton {
	return [][]models.InlineKeyboardButton{buttons}
}
