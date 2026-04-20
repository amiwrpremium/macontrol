package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// WiFi renders the 📶 Wi-Fi dashboard. features gates version-sensitive
// buttons (speedtest needs macOS 12+).
func WiFi(info wifi.Info, features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	power := "off"
	ssid := "—"
	if info.PowerOn {
		power = "on"
		if info.SSID != "" {
			ssid = info.SSID
		} else {
			ssid = "(not associated)"
		}
	}
	text = fmt.Sprintf("📶 *Wi-Fi* — `%s` · SSID `%s` · iface `%s`", power, ssid, info.Interface)

	toggle := "⏻ Turn on"
	if info.PowerOn {
		toggle = "⏻ Turn off"
	}

	rows := [][]models.InlineKeyboardButton{
		{
			{Text: toggle, CallbackData: callbacks.Encode(callbacks.NSWifi, "toggle")},
			{Text: "ℹ Info", CallbackData: callbacks.Encode(callbacks.NSWifi, "info")},
		},
		{
			{Text: "🔗 Join network…", CallbackData: callbacks.Encode(callbacks.NSWifi, "join")},
		},
		{
			{Text: "🌐 DNS → Cloudflare", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "cf")},
			{Text: "🌐 DNS → Google", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "google")},
			{Text: "🌐 DNS → DHCP", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "reset")},
		},
	}
	if features.NetworkQuality {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "⚡ Speed test", CallbackData: callbacks.Encode(callbacks.NSWifi, "speedtest")},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSWifi, "refresh")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}
