package keyboards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// System renders the 🖥 System main menu.
//
// Behavior:
//   - Static "🖥 *System*" header with no state — every
//     section's data lives behind a drill-down button.
//   - 3-row 2-button keyboard: Info / Temperature, Memory /
//     CPU, Top10 / Kill, plus the standard Back/Home nav.
//
// The pairings are deliberate:
//   - Info + Temperature = "tell me about the hardware".
//   - Memory + CPU = "tell me about utilisation".
//   - Top 10 + Kill = "pick processes to act on".
//
// Tapping any button dispatches into [handlers.handleSystem]
// at the matching action.
func System() (text string, markup *models.InlineKeyboardMarkup) {
	text = "🖥 *System*"
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "ℹ Info", CallbackData: callbacks.Encode(callbacks.NSSystem, "info")},
				{Text: "🌡 Temperature", CallbackData: callbacks.Encode(callbacks.NSSystem, "temp")},
			},
			{
				{Text: "🧠 Memory", CallbackData: callbacks.Encode(callbacks.NSSystem, "mem")},
				{Text: "⚙ CPU", CallbackData: callbacks.Encode(callbacks.NSSystem, "cpu")},
			},
			{
				{Text: "📋 Top 10 processes", CallbackData: callbacks.Encode(callbacks.NSSystem, "top")},
				{Text: "🔪 Kill process…", CallbackData: callbacks.Encode(callbacks.NSSystem, "kill")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
	return
}

// SystemPanel returns the trailing keyboard for a read-only
// System sub-panel (Temperature, but also reused by Info).
//
// Behavior:
//   - Refresh re-runs the panel's own action (passed as
//     `action` arg) so the user can re-fetch without going
//     up a level.
//   - Back returns to the System main menu (`sys:open`),
//     not to the home grid.
//   - Standard Home row from [Nav].
//
// Used by panels with no per-row content. For panels that DO
// surface tappable processes (Memory, CPU), use
// [SystemPanelWithProcs] instead.
func SystemPanel(action string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSystem, action)},
				{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSSystem, "open")},
			},
			Nav(),
		},
	}
}

// SystemPanelWithProcs renders the Memory / CPU panel
// keyboard with one tappable button per top-N process above
// the standard Refresh + Back + Home rows.
//
// Arguments:
//   - action is the panel's own callback action ("mem" or
//     "cpu"), used by Refresh.
//   - procs is the top-N slice from [system.Service.TopByMem]
//     or [system.Service.TopN]. Empty slice (e.g. ps failed)
//     collapses to the standard panel with no per-process
//     rows.
//   - labelFn formats each row's button text. Callers pick
//     between [handlers.cpuProcLabel] (PID · CPU% · leaf) or
//     [handlers.memProcLabel] (PID · MEM% · leaf) so the
//     same panel function works for both dashboards.
//
// Behavior:
//   - One button per process; tap dispatches `sys:proc:<pid>`
//     into the per-process drill-down via [SystemProcPanel].
//   - Refresh + Back + Home rows after the per-process
//     buttons.
func SystemPanelWithProcs(action string, procs []system.Process, labelFn func(system.Process) string) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(procs)+2)
	for _, p := range procs {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         labelFn(p),
			CallbackData: callbacks.Encode(callbacks.NSSystem, "proc", strconv.Itoa(p.PID)),
		}})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSystem, action)},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSSystem, "open")},
	})
	rows = append(rows, Nav())
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SystemTopList renders the Top-10 processes list page: one
// tappable button per process (PID · CPU% · leaf-of-cmd),
// then refresh + back + home.
//
// Behavior:
//   - Hard-codes the CPU% format (no labelFn arg unlike
//     [SystemPanelWithProcs]) because Top-10 is always
//     CPU-sorted.
//   - Tapping a process dispatches `sys:proc:<pid>` →
//     [SystemProcPanel] for the per-process drill-down.
//   - Refresh re-runs the Top-10 query (`sys:top`); Back
//     returns to the System menu.
//
// The leaf-of-cmd extraction (via [leafOf]) keeps long full
// paths from blowing the button width on a phone screen.
func SystemTopList(procs []system.Process) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(procs)+2)
	for _, p := range procs {
		label := fmt.Sprintf("%d · %.1f%% · %s", p.PID, p.CPU, leafOf(p.Command))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         label,
			CallbackData: callbacks.Encode(callbacks.NSSystem, "proc", strconv.Itoa(p.PID)),
		}})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSystem, "top")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSSystem, "open")},
	})
	rows = append(rows, Nav())
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SystemProcPanel renders the per-process drill-down keyboard.
// Reached by tapping a row in [SystemTopList] or
// [SystemPanelWithProcs].
//
// Behavior:
//   - Row 1: Kill (SIGTERM) and Force Kill (SIGKILL). The
//     SIGTERM path dispatches `sys:kill-pid:<pid>` and runs
//     immediately. The SIGKILL path dispatches `sys:kill9:<pid>`
//     and routes through [SystemKillConfirm] before
//     executing.
//   - Row 2: Refresh (re-fetches this same drill-down) and
//     "← Back to Top" (returns to the Top-10 list, NOT the
//     System menu — preserves the user's place).
//   - Standard Home row.
func SystemProcPanel(pid int) *models.InlineKeyboardMarkup {
	p := strconv.Itoa(pid)
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔪 Kill (SIGTERM)", CallbackData: callbacks.Encode(callbacks.NSSystem, "kill-pid", p)},
				{Text: "💀 Force Kill", CallbackData: callbacks.Encode(callbacks.NSSystem, "kill9", p)},
			},
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSSystem, "proc", p)},
				{Text: "← Back to Top", CallbackData: callbacks.Encode(callbacks.NSSystem, "top")},
			},
			Nav(),
		},
	}
}

// SystemKillConfirm renders the SIGKILL confirmation page —
// a destructive-action gate with the same shape as
// [PowerConfirm] but per-process.
//
// Arguments:
//   - pid is the target process ID, embedded in both buttons'
//     callbacks and shown verbatim in the header.
//   - name is the user-visible process name from
//     [handlers.procNameByPID]. Empty when the PID isn't in
//     the current Top-10 (process exited or never appeared);
//     the header substitutes "(unknown)" so the user always
//     has SOMETHING to read.
//
// Behavior:
//   - Header: "⚠ *Force kill PID N* (`<name>`)?" + warning
//     about the no-cleanup nature of SIGKILL.
//   - Confirm dispatches `sys:kill9:<pid>:ok` (the "ok" arg
//     is what [handlers.isConfirm] checks for).
//   - Cancel dispatches `sys:proc:<pid>` — returns to the
//     per-process drill-down so the user can re-pick the
//     polite SIGTERM instead. NOT to the System menu —
//     preserves intent.
func SystemKillConfirm(pid int, name string) (text string, markup *models.InlineKeyboardMarkup) {
	display := name
	if display == "" {
		display = "(unknown)"
	}
	text = fmt.Sprintf("⚠ *Force kill PID %d* (`%s`)?\n\nThis sends SIGKILL — the process can't clean up.",
		pid, display)
	p := strconv.Itoa(pid)
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: callbacks.Encode(callbacks.NSSystem, "kill9", p, "ok")},
				{Text: "✖ Cancel", CallbackData: callbacks.Encode(callbacks.NSSystem, "proc", p)},
			},
		},
	}
	return
}

// leafOf returns the basename of a slash-separated command
// path — equivalent to filepath.Base but without importing
// path/filepath for one tiny use.
//
// Behavior:
//   - "/Applications/Foo.app/Contents/MacOS/Foo" → "Foo".
//   - "WindowServer" (no slash) → "WindowServer".
//   - "" → "".
//
// Mirrors [handlers.leafOfPath] in the handlers package; the
// duplication is intentional to keep both packages
// dependency-free of each other (handlers imports keyboards;
// the reverse would create a cycle). See the smell on
// handlers/sys.go for the consolidation idea.
func leafOf(cmd string) string {
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		return cmd[i+1:]
	}
	return cmd
}
