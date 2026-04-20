package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleSystem(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.System

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.System()
		return r.Edit(ctx, q, text, kb)

	case "info":
		r.Ack(ctx, q)
		info, err := svc.Info(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🖥 *System info* — unavailable", err)
		}
		body := fmt.Sprintf(
			"🖥 *System info*\n\n• %s %s (%s)\n• Host: `%s`\n• Model: `%s`\n• Chip: `%s`\n• Cores: `%s`\n• RAM: `%s`\n• %s",
			info.ProductName, info.ProductVersion, info.BuildVersion,
			info.Hostname, info.Model, info.ChipName, info.CPUCores,
			fmtBytes(info.TotalRAMBytes), info.Uptime,
		)
		return r.Edit(ctx, q, body, keyboards.SystemPanel("info"))

	case "temp":
		r.Ack(ctx, q)
		t, err := svc.Thermal(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🌡 *Thermal* — unavailable", err)
		}
		var body strings.Builder
		body.WriteString(fmt.Sprintf("🌡 *Thermal*\n\n• Pressure: `%s`", t.Pressure))
		if t.SmctempAvail {
			body.WriteString(fmt.Sprintf("\n• CPU: `%.1f°C`\n• GPU: `%.1f°C`", t.CPUTempC, t.GPUTempC))
		} else {
			body.WriteString("\n• °C readings unavailable (install `brew install smctemp`).")
		}
		return r.Edit(ctx, q, body.String(), keyboards.SystemPanel("temp"))

	case "mem":
		r.Ack(ctx, q)
		m, err := svc.Memory(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🧠 *Memory* — unavailable", err)
		}
		body := "🧠 *Memory*\n\n"
		if m.PhysMemSummary != "" {
			body += Code(m.PhysMemSummary) + "\n"
		}
		if m.PressureLevel != "" {
			body += "• " + m.PressureLevel + "\n"
		}
		return r.Edit(ctx, q, body, keyboards.SystemPanel("mem"))

	case "cpu":
		r.Ack(ctx, q)
		c, err := svc.CPU(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "⚙ *CPU* — unavailable", err)
		}
		body := fmt.Sprintf("⚙ *CPU*\n\n• `%s`\n• `%s`", c.TopHeader, c.LoadAverage)
		return r.Edit(ctx, q, body, keyboards.SystemPanel("cpu"))

	case "top":
		r.Ack(ctx, q)
		procs, err := svc.TopN(ctx, 10)
		if err != nil {
			return errEdit(ctx, r, q, "📋 *Top* — unavailable", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%-6s %5s %5s  %s\n", "PID", "%CPU", "%MEM", "CMD"))
		for _, p := range procs {
			b.WriteString(fmt.Sprintf("%-6d %5.1f %5.1f  %s\n", p.PID, p.CPU, p.Mem, p.Command))
		}
		return r.Edit(ctx, q, "📋 *Top 10 by CPU*\n"+Code(b.String()), keyboards.SystemPanel("top"))

	case "kill":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewKillProc(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown system action.")
	return nil
}

func fmtBytes(n uint64) string {
	const GiB = 1 << 30
	if n == 0 {
		return "?"
	}
	return fmt.Sprintf("%.0f GiB", float64(n)/float64(GiB))
}
