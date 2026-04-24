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

// handleSystem is the System dashboard's callback dispatcher.
// Reached via the [callbacks.NSSystem] namespace from any tap on
// the 🖥 System menu and its drill-downs.
//
// Routing rules (data.Action — first match wins):
//  1. "open"      → render the System main menu via [keyboards.System].
//  2. "info"      → run [system.Service.Info], render the OS+HW
//     panel with parsed labels via [writeUptimeBlock].
//  3. "temp"      → run [system.Service.Thermal], render the
//     pressure + smctemp °C panel.
//  4. "mem"       → run [system.Service.Memory] + Info, render
//     the labelled memory panel via [buildMemoryPanel] with a
//     top-3 RAM hogs row from
//     [keyboards.SystemPanelWithProcs].
//  5. "cpu"       → run [system.Service.CPU] + Info, render the
//     labelled CPU panel via [buildCPUPanel] with a top-3 CPU
//     hogs row.
//  6. "top"       → run [system.Service.TopN] for top-10, render
//     the tappable process list via [keyboards.SystemTopList].
//  7. "proc"      → drill into one process by PID; show
//     "not in current Top 10" message when the PID has fallen
//     off the list.
//  8. "kill-pid"  → SIGTERM the PID, then re-render the Top 10
//     with a confirmation banner via [rerenderTopWithToast].
//  9. "kill9"     → confirm-then-SIGKILL flow. First tap renders
//     the [keyboards.SystemKillConfirm] dialog; second tap
//     (with "ok" in args) executes.
//  10. "kill"     → install the typed-PID kill flow
//     ([flows.NewKillProc]) for users whose target isn't in
//     the current Top 10.
//
// Unknown actions fall through to a "Unknown system action."
// toast. Errors from any sub-step are surfaced via [errEdit] so
// the user sees the macOS CLI's own diagnostic.
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
			keyboards.SystemPanelWithProcs("mem", m.TopByMem, memProcLabel))

	case "cpu":
		r.Ack(ctx, q)
		c, err := svc.CPU(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "⚙ *CPU* — unavailable", err)
		}
		info, _ := svc.Info(ctx)
		return r.Edit(ctx, q, buildCPUPanel(c, info.CPUCores),
			keyboards.SystemPanelWithProcs("cpu", c.TopByCPU, cpuProcLabel))

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

// pidArg extracts a positive integer PID from data.Args[0].
//
// Behavior:
//   - Returns (0, false) when data.Args is empty.
//   - Returns (0, false) when the arg is non-numeric or
//     non-positive (PIDs are always >= 1; PID 0 is the kernel
//     scheduler and rejecting it defends against the obvious
//     "kill 0" footgun).
//   - Returns (n, true) on success.
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

// findProc looks up the current Top-10 process snapshot and
// returns the matching [system.Process].
//
// Behavior:
//   - Calls [system.Service.TopN] with n=10. On TopN error,
//     returns ({}, false) — no propagation, callers either
//     render a generic "not in Top 10" message or fall through.
//   - Walks the result for an exact PID match.
//   - Returns (process, true) on hit, ({}, false) when the PID
//     either exited between the original tap and this lookup,
//     or never appeared in the list (typed-PID flow target).
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

// procNameByPID returns the basename-of-command for a PID via
// [findProc] + [leafOfPath]. Returns "" when the PID isn't in
// the current Top 10. Used by the kill-confirm dialog so the
// user sees a process name in the prompt instead of a bare PID.
func procNameByPID(ctx context.Context, svc *system.Service, pid int) string {
	p, ok := findProc(ctx, svc, pid)
	if !ok {
		return ""
	}
	return leafOfPath(p.Command)
}

// rerenderTopWithToast re-renders the Top-10 list with a status
// banner stamped above the list. Used after a successful kill
// so the user sees what happened in the same edit-in-place
// message rather than a separate dispatched reply.
//
// Behavior:
//   - Re-runs [system.Service.TopN] for the post-kill snapshot
//     (the just-killed process should be gone or in zombie
//     state).
//   - Stamps the supplied msg below the "Top 10 by CPU" header.
//   - Edits the original message via [Reply.Edit] with the
//     fresh [keyboards.SystemTopList] keyboard.
func rerenderTopWithToast(ctx context.Context, r Reply, q *models.CallbackQuery,
	svc *system.Service, msg string,
) error {
	procs, _ := svc.TopN(ctx, 10)
	text := "📋 *Top 10 by CPU*\n\n" + msg
	return r.Edit(ctx, q, text, keyboards.SystemTopList(procs))
}

// leafOfPath returns the basename of a slash-separated command
// path. Mirrors keyboards.leafOf so the handler text and the
// keyboard's button labels agree on the displayed name.
//
// Behavior:
//   - For "/usr/local/bin/foo" → "foo".
//   - For "WindowServer" (no slash) → "WindowServer".
//   - For "/" or "" → "" / unchanged.
func leafOfPath(cmd string) string {
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		return cmd[i+1:]
	}
	return cmd
}

// fmtBytes renders a byte count as a coarse "%.0f GiB" string
// for the System info line. Returns "?" for zero (unknown).
//
// Note: deliberately rounds to 0 decimal places; the System
// info panel doesn't need fractional GiB precision. For finer
// rendering see [humanBytes].
func fmtBytes(n uint64) string {
	const GiB = 1 << 30
	if n == 0 {
		return "?"
	}
	return fmt.Sprintf("%.0f GiB", float64(n)/float64(GiB))
}

// buildCPUPanel renders the ⚙ CPU dashboard panel: parsed
// busy/idle percentages, load averages with per-core utilisation,
// and a top-3 CPU hogs banner when [system.CPU.TopByCPU] is
// non-empty.
//
// Behavior:
//   - When any of UserPct / SysPct / IdlePct is > 0, renders
//     the labelled busy/idle line. Otherwise falls back to the
//     verbatim [system.CPU.Raw] line so the user sees SOMETHING.
//   - When any load-avg is > 0, renders the 1/5/15m triple.
//     If [system.FirstInt] can pull a core count out of
//     cpuCores, also renders per-core percentage estimates.
//   - When TopByCPU has entries, appends an italic "Top by
//     CPU — tap a process to drill in:" footer that pairs with
//     the keyboard's per-row buttons.
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
		b.WriteString("\n\n_Top by CPU — tap a process to drill in:_")
	}
	return b.String()
}

// cpuProcLabel formats a [system.Process] as a CPU-panel
// per-row button label: "<PID> · <CPU>% · <leaf-of-cmd>".
// Mirrors the SystemTopList button shape so the visual
// language is consistent between dashboards.
func cpuProcLabel(p system.Process) string {
	return fmt.Sprintf("%d · %.1f%% · %s", p.PID, p.CPU, leafOfPath(p.Command))
}

// memProcLabel is the Memory-panel equivalent of [cpuProcLabel]:
// uses %MEM instead of %CPU. Same "<PID> · <pct>% · <leaf>" shape.
func memProcLabel(p system.Process) string {
	return fmt.Sprintf("%d · %.1f%% · %s", p.PID, p.Mem, leafOfPath(p.Command))
}

// buildMemoryPanel renders the 🧠 Memory dashboard panel:
// parsed values labelled with brief jargon explanations, swap
// stats, pressure label, and a top-3 RAM hogs banner when
// [system.Memory.TopByMem] is non-empty.
//
// Behavior:
//   - When UsedBytes or totalRAMBytes is > 0, renders the
//     "Used / Total (pct)" line plus Free. Otherwise falls
//     back to the verbatim [system.Memory.Raw] line.
//   - WiredBytes / CompressedBytes are added with their italic
//     jargon-explanation suffix when present.
//   - SwapUsedBytes / SwapTotalBytes line is added when
//     SwapTotalBytes > 0.
//   - FreePercent is rendered with a [pressureLabel] mapping
//     when >= 0 (sentinel -1 means unknown).
//   - When TopByMem has entries, appends an italic "Top by
//     RAM — tap a process to drill in:" footer.
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
		b.WriteString("\n\n_Top by RAM — tap a process to drill in:_")
	}
	return b.String()
}

// pressureLabel maps a free-memory percentage to a coarse
// human label using rough thresholds.
//
// Routing rules (first match wins):
//  1. >= 30 → "Normal"
//  2. >= 10 → "Warning"
//  3. < 10  → "Critical"
//
// Thresholds are heuristic, not Apple-defined. Tuned for the
// Mac dashboard "is it bad?" intuition, not for production-style
// alerting.
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

// humanBytes renders a byte count as a sensible-magnitude
// human string, switching between MiB and GiB based on size.
//
// Behavior:
//   - n == 0 → "?" (treat zero as "unknown" because every
//     caller of this helper has a sentinel-zero "field
//     wasn't populated" semantics).
//   - n >= 1 GiB → "%.1f GiB" (one decimal so 1.5 GiB doesn't
//     round to 2).
//   - else → "%.0f MiB" (no decimals; sub-MiB precision is
//     noise on a memory dashboard).
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

// percentOf returns 100 * num / denom as an int, guarded
// against zero-denom (returns 0) and clamped to <= 100 (defends
// against numerator > denominator anomalies that Go's uint
// arithmetic would otherwise overflow into a huge percentage).
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

// writeUptimeBlock appends labelled uptime / users / load-avg
// bullets to b for the System info panel.
//
// Behavior:
//   - When all three load averages are zero, parsing failed
//     completely — falls back to appending the raw [system.Uptime.Raw]
//     line so the user sees SOMETHING. Returns immediately.
//   - Otherwise appends Uptime + Logged-in users (with English
//     plural via the noun switch) + Load avg lines. The Load
//     avg line gains per-core percentage estimates when
//     [system.FirstInt] can pull a core count out of cpuCores.
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
