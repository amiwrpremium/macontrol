package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// ToolsDiskRow is one entry on the Disks list page. ShortID is a
// callbacks.ShortMap-issued opaque id for the mount path so we don't
// blow the 64-byte callback_data limit on long /Volumes/ paths.
type ToolsDiskRow struct {
	Mount    string // for the button label
	Size     string // human form, e.g. "460Gi"
	Capacity string // e.g. "54%"
	ShortID  string // map id resolved by handler
}

// Tools renders the 🛠 Tools menu. features gates the Shortcuts runner
// (needs macOS 13+).
func Tools(features capability.Features) (text string, markup *models.InlineKeyboardMarkup) {
	text = "🛠 *Tools*"
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "📋 Clipboard (read)", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "get")},
			{Text: "📋 Clipboard (set)…", CallbackData: callbacks.Encode(callbacks.NSTools, "clip", "set")},
		},
		{
			{Text: "🧭 Timezone…", CallbackData: callbacks.Encode(callbacks.NSTools, "tz")},
			{Text: "🔄 Sync time", CallbackData: callbacks.Encode(callbacks.NSTools, "synctime")},
		},
		{
			{Text: "💿 Disks", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
		},
	}
	if features.Shortcuts {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "⚡ Run Shortcut…", CallbackData: callbacks.Encode(callbacks.NSTools, "shortcut")},
		})
	}
	rows = append(rows, NavWithBack(callbacks.NSNav, "home"))
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// ToolsDisksList renders the 💿 Disks list page: one button per
// user-facing mount (label "<mount> · <size> · <cap> used"), then
// the standard Refresh / ← Back / 🏠 Home rows. Tap drills into
// ToolsDiskPanel via tls:disk:<shortID>.
func ToolsDisksList(rows []ToolsDiskRow) *models.InlineKeyboardMarkup {
	out := make([][]models.InlineKeyboardButton, 0, len(rows)+2)
	for _, d := range rows {
		out = append(out, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s · %s · %s used", d.Mount, d.Size, d.Capacity),
			CallbackData: callbacks.Encode(callbacks.NSTools, "disk", d.ShortID),
		}})
	}
	out = append(out, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSTools, "open")},
	})
	out = append(out, Nav())
	return &models.InlineKeyboardMarkup{InlineKeyboard: out}
}

// ToolsDiskPanel renders the per-disk drill-down. Open in Finder is
// always shown; Eject is gated on removable (only safe for
// /Volumes/* with Removable Media: Removable). Refresh re-runs the
// drill-down for this disk; Back returns to the disks list.
func ToolsDiskPanel(shortID string, removable bool) *models.InlineKeyboardMarkup {
	actions := []models.InlineKeyboardButton{
		{Text: "📂 Open in Finder", CallbackData: callbacks.Encode(callbacks.NSTools, "disk-open", shortID)},
	}
	if removable {
		actions = append(actions, models.InlineKeyboardButton{
			Text: "⏏ Eject", CallbackData: callbacks.Encode(callbacks.NSTools, "disk-eject", shortID),
		})
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			actions,
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSTools, "disk", shortID)},
				{Text: "← Back to Disks", CallbackData: callbacks.Encode(callbacks.NSTools, "disks")},
			},
			Nav(),
		},
	}
}
