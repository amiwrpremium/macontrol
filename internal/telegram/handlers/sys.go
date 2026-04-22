package handlers

import (
	"context"
	"fmt"
	"strconv"
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
		return r.Edit(ctx, q, "📋 *Top 10 by CPU*\n\nTap a process for actions.",
			keyboards.SystemTopList(procs))

	case "proc":
		r.Ack(ctx, q)
		pid, ok := pidArg(data)
		if !ok {
			return errEdit(ctx, r, q, "📋 *Process*", fmt.Errorf("missing or invalid PID"))
		}
		p, found := findProc(ctx, svc, pid)
		if !found {
			return r.Edit(ctx, q,
				fmt.Sprintf("📋 *PID %d* — not in current Top 10 (may have exited).", pid),
				keyboards.SystemProcPanel(pid))
		}
		body := fmt.Sprintf("📋 *%s*\nPID: `%d` · CPU: `%.1f%%` · RAM: `%.1f%%`\n`%s`",
			leafOfPath(p.Command), p.PID, p.CPU, p.Mem, p.Command)
		return r.Edit(ctx, q, body, keyboards.SystemProcPanel(pid))

	case "kill-pid":
		r.Ack(ctx, q)
		pid, ok := pidArg(data)
		if !ok {
			return errEdit(ctx, r, q, "🔪 *Kill*", fmt.Errorf("missing or invalid PID"))
		}
		if err := svc.Kill(ctx, pid); err != nil {
			return errEdit(ctx, r, q, fmt.Sprintf("🔪 *Kill PID %d* — failed", pid), err)
		}
		return rerenderTopWithToast(ctx, r, q, svc, fmt.Sprintf("✅ SIGTERM sent to PID `%d`.", pid))

	case "kill9":
		r.Ack(ctx, q)
		pid, ok := pidArg(data)
		if !ok {
			return errEdit(ctx, r, q, "💀 *Force kill*", fmt.Errorf("missing or invalid PID"))
		}
		if !isConfirm(data.Args[1:]) {
			name := procNameByPID(ctx, svc, pid)
			text, kb := keyboards.SystemKillConfirm(pid, name)
			return r.Edit(ctx, q, text, kb)
		}
		if err := svc.KillForce(ctx, pid); err != nil {
			return errEdit(ctx, r, q, fmt.Sprintf("💀 *Force kill PID %d* — failed", pid), err)
		}
		return rerenderTopWithToast(ctx, r, q, svc, fmt.Sprintf("💀 SIGKILL sent to PID `%d`.", pid))

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

// pidArg extracts a positive PID from data.Args[0].
func pidArg(data callbacks.Data) (int, bool) {
	if len(data.Args) == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(data.Args[0])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// findProc looks up the current Top 10 and returns the matching
// process. Returns ok=false if the PID isn't in the current snapshot
// (process exited or fell off the list).
func findProc(ctx context.Context, svc *system.Service, pid int) (system.Process, bool) {
	procs, err := svc.TopN(ctx, 10)
	if err != nil {
		return system.Process{}, false
	}
	for _, p := range procs {
		if p.PID == pid {
			return p, true
		}
	}
	return system.Process{}, false
}

// procNameByPID returns the leaf-of-cmd for a PID if it's in the
// current Top 10. Empty string if not found.
func procNameByPID(ctx context.Context, svc *system.Service, pid int) string {
	p, ok := findProc(ctx, svc, pid)
	if !ok {
		return ""
	}
	return leafOfPath(p.Command)
}

// rerenderTopWithToast re-renders the Top 10 list with a status line
// at the top (used after a successful kill so the user sees what
// happened without a separate dispatched message).
func rerenderTopWithToast(ctx context.Context, r Reply, q *models.CallbackQuery,
	svc *system.Service, msg string,
) error {
	procs, _ := svc.TopN(ctx, 10)
	text := "📋 *Top 10 by CPU*\n\n" + msg
	return r.Edit(ctx, q, text, keyboards.SystemTopList(procs))
}

// leafOfPath returns the basename of a command path, matching
// keyboards.leafOf so the handler text and button labels agree.
func leafOfPath(cmd string) string {
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		return cmd[i+1:]
	}
	return cmd
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
