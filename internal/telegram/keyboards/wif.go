package keyboards

import (
	"fmt"
	"strings"

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
	// Optional second line with rich link details (only if any of them
	// are populated — wdutil/system_profiler may not be available).
	if info.PowerOn && info.SSID != "" && (info.Security != "" || info.RSSI != 0 || info.TxRateMbps != 0 || info.Channel != "") {
		var parts []string
		if info.Security != "" {
			parts = append(parts, fmt.Sprintf("`%s`", info.Security))
		}
		if info.RSSI != 0 {
			parts = append(parts, fmt.Sprintf("`%d dBm`", info.RSSI))
		}
		if info.TxRateMbps != 0 {
			parts = append(parts, fmt.Sprintf("`%g Mbps`", info.TxRateMbps))
		}
		if info.Channel != "" {
			parts = append(parts, fmt.Sprintf("ch `%s`", info.Channel))
		}
		text += "\n" + strings.Join(parts, " · ")
	}

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

// WiFiDiagPanel renders the trailing keyboard for the diagnostics
// drill-down (sudo wdutil info dump): refresh the same view, or back
// to the main Wi-Fi dashboard.
func WiFiDiagPanel() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSWifi, "info")},
				{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSWifi, "open")},
			},
			Nav(),
		},
	}
}
