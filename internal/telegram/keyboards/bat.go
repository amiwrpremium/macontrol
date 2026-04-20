package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// Battery renders the 🔋 Battery dashboard.
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
			Nav(),
		},
	}
	return
}

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
