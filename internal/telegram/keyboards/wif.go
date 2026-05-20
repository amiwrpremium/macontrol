package keyboards

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// WiFi renders the 📶 Wi-Fi dashboard.
//
// Arguments:
//   - info is the current [wifi.Info] snapshot. The renderer
//     gracefully handles partial info (Wi-Fi off, not
//     associated, missing rich-link fields).
//   - features gates version-sensitive buttons. Currently only
//     [capability.Features.NetworkQuality] (Speed test, macOS
//     12+) is gated; everything else works on macOS 11+.
//
// Behavior:
//
// Header rendering:
//  1. Power state: "on" or "off" based on info.PowerOn.
//  2. SSID: info.SSID when associated, "(not associated)" when
//     PowerOn but no SSID, "—" when PowerOn=false.
//  3. Interface: info.Interface verbatim.
//  4. Optional second line with rich link details (Security,
//     RSSI, TxRateMbps, Channel) ONLY when:
//     a) PowerOn is true AND
//     b) SSID is non-empty AND
//     c) at least one rich field is non-zero/non-empty.
//     Each present field becomes a "·"-separated chip.
//
// Keyboard rendering:
//   - Toggle button (Turn on / Turn off based on PowerOn).
//   - Info button (drills into [WiFiDiagPanel]).
//   - Join network… button.
//   - DNS… button (drills into [WiFiDNS] submenu — collapses
//     the previous triple-button DNS preset row, see PR #65).
//   - Speed test button (only when features.NetworkQuality).
//   - Refresh row.
//   - Back/Home nav row.
func WiFi(info wifi.Info, features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	text = wifiHeader(info)
	if extra := wifiDetailLine(info); extra != "" {
		text += "\n" + extra
	}
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: wifiRows(info, features)}
	return
}

// wifiHeader builds the first line of the Wi-Fi dashboard:
// power + SSID + interface name.
func wifiHeader(info wifi.Info) string {
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
	return fmt.Sprintf("📶 *Wi-Fi* — `%s` · SSID `%s` · iface `%s`", power, ssid, info.Interface)
}

// wifiDetailLine builds the optional second line with rich link
// details (Security / RSSI / Tx rate / channel). Returns "" when
// Wi-Fi is off, the radio isn't associated, or no detail field
// is populated.
func wifiDetailLine(info wifi.Info) string {
	if !info.PowerOn || info.SSID == "" {
		return ""
	}
	parts := wifiDetailParts(info)
	return strings.Join(parts, " · ")
}

// wifiDetailParts returns the slice of formatted detail tokens
// (Security / RSSI / Tx rate / channel), each gated on its
// respective field being set. Empty slice when nothing populated
// — wifiDetailLine then returns "" via the no-op Join.
func wifiDetailParts(info wifi.Info) []string {
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
	return parts
}

// wifiRows builds the keyboard row list, including the optional
// Speed test row when the macOS networkQuality CLI is present.
func wifiRows(info wifi.Info, features capability.Features) [][]models.InlineKeyboardButton {
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
			{Text: "🌐 DNS…", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns-menu")},
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
	rows = append(rows, NavWithBack(callbacks.NSNav, "home"))
	return rows
}

// WiFiDNS renders the DNS-preset submenu reached by tapping
// "🌐 DNS…" on the main Wi-Fi dashboard. Three preset buttons
// (Cloudflare, Google, DHCP reset), then Refresh + Back-to-Wi-Fi
// + Home.
//
// Behavior:
//   - Returns a static header pointing at the current Wi-Fi
//     interface ("Pick a preset — it applies instantly").
//   - Each preset button keeps the original `wif:dns:<preset>`
//     callback shape from PR #65 — the handler branch is
//     unchanged from when the presets lived as direct buttons
//     on the main dashboard. This means [Service.SetDNS] sees
//     the same call regardless of how the user got there.
//   - Refresh re-renders this menu (`wif:dns-menu`); Back
//     returns to the Wi-Fi dashboard (`wif:open`).
func WiFiDNS() (text string, markup *models.InlineKeyboardMarkup) {
	text = "🌐 *DNS servers*\n\nPick a preset — it applies instantly to the active Wi-Fi interface."
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "☁ Cloudflare (1.1.1.1)", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "cf")}},
			{{Text: "🅖 Google (8.8.8.8)", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "google")}},
			{{Text: "↺ Reset to DHCP", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns", "reset")}},
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSWifi, "dns-menu")},
				{Text: "← Back to Wi-Fi", CallbackData: callbacks.Encode(callbacks.NSWifi, "open")},
			},
			Nav(),
		},
	}
	return
}

// WiFiDiagPanel renders the trailing keyboard for the
// diagnostics drill-down opened by the "ℹ Info" button. The
// drill-down body is the raw `sudo wdutil info` dump rendered
// in a code block; this keyboard is what appears below it.
//
// Behavior:
//   - Refresh re-runs the diagnostics dump (`wif:info`).
//   - Back returns to the Wi-Fi dashboard (`wif:open`).
//   - Standard Home row.
//
// Notably absent: any "Toggle" / "Join" buttons. The user is in
// drill-down mode here; the keyboard is intentionally minimal
// to avoid accidental state changes from the diagnostic view.
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
