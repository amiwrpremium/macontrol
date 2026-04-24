package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleWiFi is the Wi-Fi dashboard's callback dispatcher.
// Reached via the [callbacks.NSWifi] namespace from any tap on
// the 📶 Wi-Fi menu, the DNS submenu, the Info diagnostics
// drill-down, or the speed-test action.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → run [wifi.Service.Get], render the
//     main Wi-Fi dashboard via [keyboards.WiFi]. Both actions
//     share the same code path; "refresh" is the user-tap entry
//     point, "open" is the dispatched-from-home entry point.
//  2. "toggle" → run [wifi.Service.Toggle] (which composes
//     Get + SetPower), render the post-toggle dashboard.
//  3. "info" → run [wifi.Service.Diag] for the verbatim
//     `sudo wdutil info` dump; render in a code block via
//     [Code] with [keyboards.WiFiDiagPanel] underneath.
//  4. "dns-menu" → render the DNS submenu via [keyboards.WiFiDNS].
//     Pure UX navigation; no Wi-Fi state changes.
//  5. "dns" → apply a DNS preset (cf / google / reset) via
//     [wifi.Service.SetDNS], then re-render the main Wi-Fi
//     dashboard with a "DNS updated → <preset>" suffix.
//  6. "speedtest" → gated on [capability.Features.NetworkQuality];
//     toasts "needs macOS 12+" when the gate is false. Otherwise
//     toasts "Running — takes ~15s…" then invokes
//     [wifi.Service.Speedtest] (which extends ctx to 60s
//     internally) and renders the result panel.
//  7. "join" → install the [flows.NewJoinWifi] two-step flow
//     for SSID + password.
//
// Unknown actions fall through to a "Unknown wifi action."
// toast. Errors from any sub-step are surfaced via [errEdit] so
// the user sees the macOS CLI's own diagnostic (e.g. the
// "sudo: a password is required" hint when the sudoers entry
// for wdutil is missing).
// wifiDispatch maps Wi-Fi callback actions to per-action handlers.
// "open" and "refresh" share a handler because both just re-render
// the main dashboard.
var wifiDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":      handleWiFiRefresh,
	"refresh":   handleWiFiRefresh,
	"toggle":    handleWiFiToggle,
	"info":      handleWiFiInfo,
	"dns-menu":  handleWiFiDNSMenu,
	"dns":       handleWiFiDNS,
	"speedtest": handleWiFiSpeedtest,
	"join":      handleWiFiJoin,
}

func handleWiFi(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := wifiDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown wifi action.")
		return nil
	}
	return h(ctx, d, q, data)
}

func handleWiFiRefresh(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	info, err := d.Services.WiFi.Get(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "📶 *Wi-Fi* — unavailable", err)
	}
	text, kb := keyboards.WiFi(info, d.Capability.Features)
	return r.Edit(ctx, q, text, kb)
}

func handleWiFiToggle(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	info, err := d.Services.WiFi.Toggle(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "📶 *Wi-Fi* — toggle failed", err)
	}
	text, kb := keyboards.WiFi(info, d.Capability.Features)
	return r.Edit(ctx, q, text, kb)
}

func handleWiFiInfo(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	out, err := d.Services.WiFi.Diag(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "📶 *Wi-Fi info* — unavailable", err)
	}
	return r.Edit(ctx, q, "📶 *Wi-Fi diagnostics*\n"+Code(truncate(out, 3500)), keyboards.WiFiDiagPanel())
}

func handleWiFiDNSMenu(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	text, kb := keyboards.WiFiDNS()
	return r.Edit(ctx, q, text, kb)
}

// dnsPresets maps the DNS submenu's preset name to the server
// list to install. The "reset" preset uses nil to mean "hand
// DNS control back to DHCP".
var dnsPresets = map[string][]string{
	"cf":     {"1.1.1.1", "1.0.0.1"},
	"google": {"8.8.8.8", "8.8.4.4"},
	"reset":  nil,
}

func handleWiFiDNS(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.WiFi
	feat := d.Capability.Features
	r.Ack(ctx, q)
	preset := "reset"
	if len(data.Args) > 0 {
		preset = data.Args[0]
	}
	servers := dnsPresets[preset]
	if err := svc.SetDNS(ctx, servers); err != nil {
		return errEdit(ctx, r, q, "📶 *DNS* — update failed", err)
	}
	info, _ := svc.Get(ctx)
	text, kb := keyboards.WiFi(info, feat)
	return r.Edit(ctx, q, text+fmt.Sprintf("\n\n_DNS updated → %s_", preset), kb)
}

func handleWiFiSpeedtest(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.WiFi
	feat := d.Capability.Features
	if !feat.NetworkQuality {
		r.Toast(ctx, q, "Speedtest needs macOS 12+")
		return nil
	}
	r.Toast(ctx, q, "Running — takes ~15s…")
	res, err := svc.Speedtest(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "⚡ *Speedtest* — failed", err)
	}
	info, _ := svc.Get(ctx)
	_, kb := keyboards.WiFi(info, feat)
	body := fmt.Sprintf("⚡ *Speedtest*\n\n• Down: `%.1f Mbps`\n• Up: `%.1f Mbps`",
		res.DownloadMbps, res.UploadMbps)
	return r.Edit(ctx, q, body, kb)
}

func handleWiFiJoin(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	chatID := q.Message.Message.Chat.ID
	f := flows.NewJoinWifi(d.Services.WiFi)
	d.FlowReg.Install(chatID, f)
	return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
}

// truncate cuts s at n bytes and appends a "(truncated)" marker
// when s exceeds n. Used by [handleWiFi]'s "info" branch to
// keep the wdutil diagnostics dump under Telegram's ~4096-char
// message limit (3500 here leaves headroom for the surrounding
// Markdown markers + safety margin).
//
// Behavior:
//   - Returns s unchanged when len(s) <= n.
//   - Otherwise returns s[:n] with a "\n…(truncated)" suffix.
//
// Note: byte-truncation, NOT rune-truncation. A multi-byte UTF-8
// codepoint at the cut boundary will be rendered as invalid
// UTF-8 in Telegram. wdutil output is ASCII in practice so this
// hasn't bitten yet.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}
