package keyboards

import (
	"fmt"
	"strconv"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// AppsPageSize is how many app buttons fit on one page of
// [AppsList]. Matches [ShortcutsPageSize] / [TimezonesPageSize]
// for visual consistency across the bot's list-shaped pickers.
const AppsPageSize = 15

// AppListItem is one row on the 🪟 Apps list page rendered by
// [AppsList].
//
// App names are routed through [callbacks.ShortMap] so the
// 64-byte callback_data budget always fits, even for apps with
// long localised names. The handler issues a fresh ShortID for
// each app on every list refresh.
//
// Field roles:
//   - Name is the user-visible app name shown verbatim on the
//     button label (prefixed with the 🪟 emoji and a "·" hidden
//     marker when applicable).
//   - PID is the kernel process id; not stamped into this
//     button's callback (Quit / Hide go by name; Force Quit
//     gets the PID from the per-app panel) but kept on the row
//     so the handler can pass it to [AppPanel] without a second
//     lookup.
//   - Hidden flips the row's leading marker from " " to "·"
//     so the user can see at a glance which apps are Cmd-H'd.
//   - ShortID is the [callbacks.ShortMap]-issued opaque id
//     that resolves to the app name on tap.
type AppListItem struct {
	// Name is the user-visible application name shown on the
	// button label.
	Name string

	// PID is the kernel process id of the running instance.
	PID int

	// Hidden is true when the app's windows are hidden (Cmd-H
	// state). Used only for the "·" leading marker on the
	// button label.
	Hidden bool

	// ShortID is the [callbacks.ShortMap]-issued opaque id
	// resolving to the app name on tap.
	ShortID string
}

// AppsList renders the paginated 🪟 Apps list page: one
// tappable button per running application, then the standard
// pager / "Quit all except…" / Refresh / Back / Home rows.
//
// Arguments:
//   - items is the page slice (already paginated by the caller).
//   - page is the 0-indexed page number for the header.
//   - totalPages is the total page count (always rendered as ≥1
//     via [atLeastOne] so an empty list still shows "Page 1/1").
//   - total is the count of running apps, shown in the header.
//
// Behavior:
//   - Each app row dispatches `app:show:<shortID>` into the
//     per-app drill-down.
//   - The pager row appears only when totalPages > 1, and only
//     includes the relevant arrow per page edge.
//   - The "🚮 Quit all except…" row dispatches `app:keep` to
//     start the multi-select checklist.
//   - Refresh re-runs `app:open` (which re-fetches the listing);
//     Back returns to the home grid.
//   - When total == 0 the body collapses to "_No running apps._"
//     so the user gets a hint instead of a blank screen.
func AppsList(items []AppListItem, page, totalPages, total int) (text string, markup *models.InlineKeyboardMarkup) {
	header := fmt.Sprintf("🪟 *Apps*  ·  Page %d/%d  ·  %d running",
		page+1, atLeastOne(totalPages), total)
	if total == 0 {
		header += "\n\n_No running apps._"
	}
	text = header

	rows := make([][]models.InlineKeyboardButton, 0, len(items)+4)
	for _, it := range items {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         appListButtonLabel(it),
			CallbackData: callbacks.Encode(callbacks.NSApps, "show", it.ShortID),
		}})
	}

	if totalPages > 1 {
		nav := make([]models.InlineKeyboardButton, 0, 2)
		if page > 0 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "← Prev",
				CallbackData: callbacks.Encode(callbacks.NSApps, "list", strconv.Itoa(page-1)),
			})
		}
		if page < totalPages-1 {
			nav = append(nav, models.InlineKeyboardButton{
				Text:         "Next →",
				CallbackData: callbacks.Encode(callbacks.NSApps, "list", strconv.Itoa(page+1)),
			})
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}

	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🚮 Quit all except…", CallbackData: callbacks.Encode(callbacks.NSApps, "keep")},
	})
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSApps, "open")},
		{Text: "← Back", CallbackData: callbacks.Encode(callbacks.NSNav, "home")},
	})
	rows = append(rows, Nav())
	markup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}
	return
}

// appListButtonLabel composes the per-row button text for
// [AppsList]. Hidden apps get a "·" leading marker so the user
// can spot Cmd-H'd apps at a glance without leaving the list.
func appListButtonLabel(it AppListItem) string {
	if it.Hidden {
		return "🪟 · " + it.Name
	}
	return "🪟 " + it.Name
}

// AppPanel renders the per-app drill-down keyboard. Reached by
// tapping a row in [AppsList].
//
// Arguments:
//   - name is the app name shown verbatim in the header.
//   - pid is the kernel process id, shown in the header so the
//     user can match the row against Activity Monitor when in
//     doubt.
//   - shortID is the [callbacks.ShortMap]-issued id stamped
//     into every action button so the handler can resolve the
//     name on tap.
//
// Behavior:
//   - Row 1: Quit (graceful) and Force Quit (SIGKILL). Both
//     route through their respective confirm pages
//     ([AppQuitConfirm] / [AppForceConfirm]) before executing.
//   - Row 2: Hide (Cmd-H equivalent). No confirm — the action
//     is reversible (Cmd-Tab brings the app back).
//   - Row 3: Refresh (re-fetches this drill-down) and "← Back
//     to apps" (returns to the list, NOT the home grid —
//     preserves the user's place).
//   - Standard 🏠 Home row.
func AppPanel(name string, pid int, shortID string) (text string, markup *models.InlineKeyboardMarkup) {
	text = fmt.Sprintf("🪟 *%s* · PID `%d`", name, pid)
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🛑 Quit", CallbackData: callbacks.Encode(callbacks.NSApps, "quit", shortID)},
				{Text: "💀 Force Quit", CallbackData: callbacks.Encode(callbacks.NSApps, "force", shortID)},
			},
			{
				{Text: "🙈 Hide", CallbackData: callbacks.Encode(callbacks.NSApps, "hide", shortID)},
			},
			{
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSApps, "show", shortID)},
				{Text: "← Back to apps", CallbackData: callbacks.Encode(callbacks.NSApps, "open")},
			},
			Nav(),
		},
	}
	return
}

// AppQuitConfirm renders the graceful-quit confirmation page
// for a single app. Reached when the user taps Quit on
// [AppPanel].
//
// Behavior:
//   - Header: "🛑 *Quit <name>*?" with a one-line note that
//     unsaved-document dialogs may keep the app alive.
//   - Confirm dispatches `app:quit:<shortID>:ok` (the "ok" arg
//     is the [handlers.isConfirm] sentinel).
//   - Cancel dispatches `app:show:<shortID>` — returns to the
//     per-app drill-down so the user can Force Quit instead.
//     NOT to the list — preserves intent.
func AppQuitConfirm(name, shortID string) (text string, markup *models.InlineKeyboardMarkup) {
	text = fmt.Sprintf("🛑 *Quit %s*?\n\nGraceful — the app may show an unsaved-document dialog and stay open.", name)
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: callbacks.Encode(callbacks.NSApps, "quit", shortID, "ok")},
				{Text: "✖ Cancel", CallbackData: callbacks.Encode(callbacks.NSApps, "show", shortID)},
			},
		},
	}
	return
}

// AppForceConfirm renders the SIGKILL confirmation page for a
// single app. Reached when the user taps Force Quit on
// [AppPanel].
//
// Arguments:
//   - name is the app name shown verbatim in the header.
//   - pid is the kernel process id; shown in the header so the
//     user has positive identification before the destructive
//     action runs.
//   - shortID is the [callbacks.ShortMap] id used by the
//     Cancel button to route back to the per-app drill-down.
//
// Behavior:
//   - Header: stronger warning than [AppQuitConfirm] —
//     SIGKILL skips cleanup, so unsaved work is gone.
//   - Confirm dispatches `app:force:<shortID>:ok` (the "ok"
//     arg is the [handlers.isConfirm] sentinel). The handler
//     resolves shortID → name → PID before sending the kill.
//   - Cancel dispatches `app:show:<shortID>` — returns to the
//     per-app drill-down so the user can pick the polite Quit
//     instead.
func AppForceConfirm(name string, pid int, shortID string) (text string, markup *models.InlineKeyboardMarkup) {
	text = fmt.Sprintf(
		"💀 *Force Quit %s* (PID `%d`)?\n\nSends SIGKILL — the app can't clean up. Unsaved work is gone.",
		name, pid,
	)
	markup = &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: callbacks.Encode(callbacks.NSApps, "force", shortID, "ok")},
				{Text: "✖ Cancel", CallbackData: callbacks.Encode(callbacks.NSApps, "show", shortID)},
			},
		},
	}
	return
}
