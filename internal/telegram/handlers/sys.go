package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
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
		var body strings.Builder
		fmt.Fprintf(&body,
			"🖥 *System info*\n\n• %s %s (%s)\n• Host: `%s`\n• Model: `%s`\n• Chip: `%s`\n• Cores: `%s`\n• RAM: `%s`",
			info.ProductName, info.ProductVersion, info.BuildVersion,
			info.Hostname, info.Model, info.ChipName, info.CPUCores,
			fmtBytes(info.TotalRAMBytes),
		)
		writeUptimeBlock(&body, info.Uptime, info.CPUCores)
		return r.Edit(ctx, q, body.String(), keyboards.SystemPanel("info"))

	case "temp":
		r.Ack(ctx, q)
		t, err := svc.Thermal(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🌡 *Thermal* — unavailable", err)
		}
		var body strings.Builder
		fmt.Fprintf(&body, "🌡 *Thermal*\n\n• Pressure: `%s`", t.Pressure)
		if t.SmctempAvail {
			fmt.Fprintf(&body, "\n• CPU: `%.1f°C`\n• GPU: `%.1f°C`", t.CPUTempC, t.GPUTempC)
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
		info, _ := svc.Info(ctx)
		return r.Edit(ctx, q, buildMemoryPanel(m, info.TotalRAMBytes),
			keyboards.SystemPanel("mem"))

	case "cpu":
		r.Ack(ctx, q)
		c, err := svc.CPU(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "⚙ *CPU* — unavailable", err)
		}
		info, _ := svc.Info(ctx)
		return r.Edit(ctx, q, buildCPUPanel(c, info.CPUCores),
			keyboards.SystemPanel("cpu"))

	case "top":
		r.Ack(ctx, q)
		procs, err := svc.TopN(ctx, 10)
		if err != nil {
			return errEdit(ctx, r, q, "📋 *Top* — unavailable", err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%-6s %5s %5s  %s\n", "PID", "%CPU", "%MEM", "CMD")
		for _, p := range procs {
			fmt.Fprintf(&b, "%-6d %5.1f %5.1f  %s\n", p.PID, p.CPU, p.Mem, p.Command)
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

// buildCPUPanel renders the ⚙ CPU panel: parsed busy/idle %, load
// averages with per-core utilisation, and a top-3 CPU hogs list when
// available. Falls back to the raw `top` "CPU usage:" line if
// parsing failed.
func buildCPUPanel(c system.CPU, cpuCores string) string {
	var b strings.Builder
	b.WriteString("⚙ *CPU*\n")

	if c.UserPct > 0 || c.SysPct > 0 || c.IdlePct > 0 {
		busy := c.UserPct + c.SysPct
		fmt.Fprintf(&b, "\n• Busy: `%.0f%%` (User `%.0f%%` · Kernel `%.0f%%`) · Idle: `%.0f%%`",
			busy, c.UserPct, c.SysPct, c.IdlePct)
	} else if c.Raw != "" {
		fmt.Fprintf(&b, "\n• %s", c.Raw)
	}
	if c.Load1 > 0 || c.Load5 > 0 || c.Load15 > 0 {
		cores, ok := system.FirstInt(cpuCores)
		if ok && cores > 0 {
			fmt.Fprintf(&b,
				"\n• Load avg (1/5/15m): `%.2f / %.2f / %.2f` (~%.0f%% / %.0f%% / %.0f%% of %d cores)",
				c.Load1, c.Load5, c.Load15,
				c.Load1/float64(cores)*100,
				c.Load5/float64(cores)*100,
				c.Load15/float64(cores)*100,
				cores)
		} else {
			fmt.Fprintf(&b, "\n• Load avg (1/5/15m): `%.2f / %.2f / %.2f`",
				c.Load1, c.Load5, c.Load15)
		}
	}
	if len(c.TopByCPU) > 0 {
		b.WriteString("\n\nTop by CPU:\n")
		var t strings.Builder
		for _, p := range c.TopByCPU {
			fmt.Fprintf(&t, "%5.1f%%  %s\n", p.CPU, p.Command)
		}
		b.WriteString(Code(strings.TrimRight(t.String(), "\n")))
	}
	return b.String()
}

// buildMemoryPanel renders the 🧠 Memory panel: parsed values
// labelled, jargon explained briefly, swap and a top-3 RAM hogs
// list when available. Falls back to the raw `top` PhysMem line if
// parsing failed.
func buildMemoryPanel(m system.Memory, totalRAMBytes uint64) string {
	var b strings.Builder
	b.WriteString("🧠 *Memory*\n")

	if m.UsedBytes > 0 || totalRAMBytes > 0 {
		fmt.Fprintf(&b, "\n• Used: `%s / %s` (%d%%) · Free: `%s`",
			humanBytes(m.UsedBytes), humanBytes(totalRAMBytes),
			percentOf(m.UsedBytes, totalRAMBytes),
			humanBytes(m.UnusedBytes))
	} else if m.Raw != "" {
		fmt.Fprintf(&b, "\n• %s", m.Raw)
	}
	if m.WiredBytes > 0 {
		fmt.Fprintf(&b, "\n• Wired: `%s` _(kernel-pinned)_", humanBytes(m.WiredBytes))
	}
	if m.CompressedBytes > 0 {
		fmt.Fprintf(&b, "\n• Compressed: `%s` _(in-RAM compression)_", humanBytes(m.CompressedBytes))
	}
	if m.SwapTotalBytes > 0 {
		fmt.Fprintf(&b, "\n• Swap used: `%s` of `%s`",
			humanBytes(m.SwapUsedBytes), humanBytes(m.SwapTotalBytes))
	}
	if m.FreePercent >= 0 {
		fmt.Fprintf(&b, "\n• Pressure: `%s` (%d%% free)",
			pressureLabel(m.FreePercent), m.FreePercent)
	}
	if len(m.TopByMem) > 0 {
		b.WriteString("\n\nTop by RAM:\n")
		var t strings.Builder
		for _, p := range m.TopByMem {
			fmt.Fprintf(&t, "%5.1f%%  %s\n", p.Mem, p.Command)
		}
		b.WriteString(Code(strings.TrimRight(t.String(), "\n")))
	}
	return b.String()
}

// pressureLabel maps a free-percentage to a human label using
// rough thresholds: <10% Critical, <30% Warning, else Normal.
func pressureLabel(freePct int) string {
	switch {
	case freePct >= 30:
		return "Normal"
	case freePct >= 10:
		return "Warning"
	default:
		return "Critical"
	}
}

// humanBytes renders a byte count as GiB or MiB depending on
// magnitude. Returns "?" for zero (i.e., unknown).
func humanBytes(n uint64) string {
	if n == 0 {
		return "?"
	}
	const (
		MiB = 1 << 20
		GiB = 1 << 30
	)
	if n >= GiB {
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	}
	return fmt.Sprintf("%.0f MiB", float64(n)/float64(MiB))
}

// percentOf returns 100 * num / denom, guarded against zero.
// Result is clamped to 0..100 to keep the int conversion safe even
// for hostile inputs.
func percentOf(num, denom uint64) int {
	if denom == 0 {
		return 0
	}
	p := num * 100 / denom
	if p > 100 {
		p = 100
	}
	return int(p)
}

// writeUptimeBlock appends labelled uptime/users/load-avg bullets to b.
// Falls back to the raw `uptime` line if parsing didn't catch the
// load triplet (the most informative field).
func writeUptimeBlock(b *strings.Builder, u system.Uptime, cpuCores string) {
	if u.Load1 == 0 && u.Load5 == 0 && u.Load15 == 0 {
		fmt.Fprintf(b, "\n• %s", u.Raw)
		return
	}
	if u.Duration != "" {
		fmt.Fprintf(b, "\n• Uptime: `%s`", u.Duration)
	}
	if u.Users > 0 {
		noun := "users"
		if u.Users == 1 {
			noun = "user"
		}
		fmt.Fprintf(b, "\n• Logged-in %s: `%d`", noun, u.Users)
	}
	cores, ok := system.FirstInt(cpuCores)
	if ok && cores > 0 {
		fmt.Fprintf(b,
			"\n• Load avg (1/5/15m): `%.2f / %.2f / %.2f` (~%.0f%% / %.0f%% / %.0f%% of %d cores)",
			u.Load1, u.Load5, u.Load15,
			u.Load1/float64(cores)*100,
			u.Load5/float64(cores)*100,
			u.Load15/float64(cores)*100,
			cores)
	} else {
		fmt.Fprintf(b, "\n• Load avg (1/5/15m): `%.2f / %.2f / %.2f`",
			u.Load1, u.Load5, u.Load15)
	}
}
