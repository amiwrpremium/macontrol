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

func handleWiFi(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.WiFi
	feat := d.Capability.Features

	switch data.Action {
	case "open", "refresh":
		r.Ack(ctx, q)
		info, err := svc.Get(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "📶 *Wi-Fi* — unavailable", err)
		}
		text, kb := keyboards.WiFi(info, feat)
		return r.Edit(ctx, q, text, kb)

	case "toggle":
		r.Ack(ctx, q)
		info, err := svc.Toggle(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "📶 *Wi-Fi* — toggle failed", err)
		}
		text, kb := keyboards.WiFi(info, feat)
		return r.Edit(ctx, q, text, kb)

	case "info":
		r.Ack(ctx, q)
		out, err := svc.Diag(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "📶 *Wi-Fi info* — unavailable", err)
		}
		return r.Edit(ctx, q, "📶 *Wi-Fi diagnostics*\n"+Code(truncate(out, 3500)), keyboards.WiFiDiagPanel())

	case "dns":
		r.Ack(ctx, q)
		var servers []string
		if len(data.Args) > 0 {
			switch data.Args[0] {
			case "cf":
				servers = []string{"1.1.1.1", "1.0.0.1"}
			case "google":
				servers = []string{"8.8.8.8", "8.8.4.4"}
			case "reset":
				servers = nil
			}
		}
		if err := svc.SetDNS(ctx, servers); err != nil {
			return errEdit(ctx, r, q, "📶 *DNS* — update failed", err)
		}
		info, _ := svc.Get(ctx)
		text, kb := keyboards.WiFi(info, feat)
		preset := "reset"
		if len(data.Args) > 0 {
			preset = data.Args[0]
		}
		return r.Edit(ctx, q, text+fmt.Sprintf("\n\n_DNS updated → %s_", preset), kb)

	case "speedtest":
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

	case "join":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewJoinWifi(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown wifi action.")
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}
