package keyboards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// System renders the 🖥 System menu. Each button opens a specific read-only
// panel (the underlying messages themselves carry text + their own refresh).
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

// SystemPanel builds a trailing refresh + nav for a sys panel (temp/mem/cpu).
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

// SystemPanelWithProcs renders the CPU/Memory panel keyboard with a
// tappable button per process above the standard Refresh + Back +
// Home rows. labelFn formats each row's text from the process —
// callers pick whether to show CPU% or RAM%. Tapping a row drills
// into SystemProcPanel via sys:proc:<pid>. When procs is empty
// (e.g. ps failed) the keyboard collapses to the standard panel.
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

// SystemTopList renders the Top 10 list page: one button per process
// (PID · CPU% · leaf-of-cmd), then refresh + back + home. Tapping a
// process drills into SystemProcPanel.
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

// SystemProcPanel renders the per-process drill-down: SIGTERM kill,
// SIGKILL force kill (confirmed), refresh, back to Top list, home.
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

// SystemKillConfirm is the SIGKILL confirmation page. Cancel returns
// to the per-process drill-down so the user can re-pick the polite
// SIGTERM if they intended that.
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

// leafOf returns the basename of a command path. "/Applications/X.app/Contents/MacOS/X"
// → "X". Bare commands without a slash come back unchanged.
func leafOf(cmd string) string {
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		return cmd[i+1:]
	}
	return cmd
}
