package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Battery renders the 🔋 Battery dashboard for the supplied
// [battery.Status] snapshot.
//
// Header rendering (first match wins):
//  1. Status.Present == false → "🔋 *Battery* — not present
//     (desktop Mac)" — no percentage / state / remaining-time
//     suffix because none apply.
//  2. Battery present → "<icon> *Battery* — `<pct>%` ·
//     <state> · `<remaining>`". The icon is picked by
//     [batteryIcon] based on percentage + charging state;
//     remaining-time falls back to "—" when pmset didn't
//     report one.
//
// Keyboard rendering (2 rows):
//  1. Refresh + Health drill-down. Health is the only nested
//     screen in the Battery category; the dashboard is
//     otherwise read-only.
//  2. Standard Back/Home nav row.
func Battery(st battery.Status) (text string, markup *models.InlineKeyboardMarkup) {
	if !st.Present {
		text = "🔋 *Battery* — not present (desktop Mac)"
	} else {
		icon := batteryIcon(st.Percent, st.State)
		rem := st.TimeRemaining
		if rem == "" {
			rem = "—"
		}
		text = fmt.Sprintf("%s *Battery* — `%d%%` · %s · `%s`", icon, st.Percent, st.State, rem)
	}
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSBattery, "refresh")},
				{Text: "📊 Health", CallbackData: callbacks.Encode(callbacks.NSBattery, "health")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}

// BatteryHealthPanel renders the trailing keyboard for the
// Battery → Health drill-down. The body itself is composed
// inline by [handlers.handleBattery]; this helper just emits
// the action row.
//
// Behavior:
//   - Refresh re-runs the health query (`bat:health`).
//   - Back returns to the main Battery dashboard
//     (`bat:open`), NOT to the home grid.
//   - Standard Home row from [Nav].
func BatteryHealthPanel() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSBattery, "health")},
				{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSBattery, "open")},
			},
			Nav(),
		},
	}
}

// batteryIcon picks an emoji glyph for the dashboard header
// based on charge level and charging state.
//
// Routing rules (first match wins):
//  1. State == [battery.StateCharging] → "⚡" (regardless of
//     percentage — a charging battery is the active state).
//  2. pct >= 80                         → "🔋" (full / near-full).
//  3. pct >= 40                         → "🔋" (mid; same
//     glyph because Telegram's icon font has no clear
//     "half full" battery emoji that renders the same on
//     iOS and Android).
//  4. pct >= 20                         → "🪫" (low).
//  5. otherwise                         → "🪫" (critical;
//     same glyph as low for the same cross-platform
//     consistency reason).
//
// The duplicated buckets ([2]+[3] and [4]+[5]) are intentional
// — when a future Unicode point ships a clearer
// "battery-half" / "battery-critical" pair, this function is
// the one place to update.
func batteryIcon(pct int, state battery.ChargeState) string {
	if state == battery.StateCharging {
		return "⚡"
	}
	switch {
	case pct >= 80:
		return "🔋"
	case pct >= 40:
		return "🔋"
	case pct >= 20:
		return "🪫"
	default:
		return "🪫"
	}
}
