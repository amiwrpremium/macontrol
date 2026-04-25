package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/apps"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleApps is the 🪟 Apps dashboard's callback dispatcher.
// Reached via the [callbacks.NSApps] namespace from any tap on
// the Apps list and its drill-downs.
//
// Routing rules (data.Action — first match wins):
//  1. "open"  → render the running-app list (page 0). Re-fetches
//     osascript output every call so the list reflects current
//     state.
//  2. "list"  → paginate. data.Args[0] is the 0-indexed page.
//  3. "show"  → per-app drill-down. data.Args[0] is the
//     [callbacks.ShortMap] id resolving to the app name; the
//     handler re-fetches the listing to confirm the app is still
//     running and to look up the PID.
//  4. "quit"  → confirm-then-Quit. First tap renders
//     [keyboards.AppQuitConfirm]; second tap (with "ok" in args)
//     executes [apps.Service.Quit] and returns to the list.
//  5. "force" → confirm-then-ForceQuit. First tap renders
//     [keyboards.AppForceConfirm]; second tap executes
//     [apps.Service.ForceQuit] and returns to the list.
//  6. "hide"  → execute [apps.Service.Hide] immediately. No
//     confirm — Cmd-H is reversible.
//
// Unknown actions fall through to a "Unknown app action."
// toast. Errors from any sub-step surface via [errEdit] so the
// user sees the macOS CLI's own diagnostic (the most common
// being "TCC denied" when Accessibility is not granted to the
// macontrol binary).
//
// Keep-related actions ("keep", "keep-toggle", "keep-confirm",
// "keep-execute", "keep-back") are wired into the same
// dispatch in commit 8 of this PR.
var appsDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":  handleAppsOpen,
	"list":  handleAppsList,
	"show":  handleAppsShow,
	"quit":  handleAppsQuit,
	"force": handleAppsForce,
	"hide":  handleAppsHide,
}

func handleApps(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := appsDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown app action.")
		return nil
	}
	return h(ctx, d, q, data)
}

func handleAppsOpen(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	return renderAppsList(ctx, r, q, d, 0)
}

func handleAppsList(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	page := 0
	if len(data.Args) > 0 {
		if p, err := strconv.Atoi(data.Args[0]); err == nil && p >= 0 {
			page = p
		}
	}
	return renderAppsList(ctx, r, q, d, page)
}

func handleAppsShow(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	name, ok := resolveAppName(d, data)
	if !ok {
		return errEdit(ctx, r, q, "🪟 *App*", fmt.Errorf("session expired — refresh the apps list"))
	}
	pid, found, err := lookupAppPID(ctx, d, name)
	if err != nil {
		return errEdit(ctx, r, q, fmt.Sprintf("🪟 *%s* — unavailable", name), err)
	}
	if !found {
		return r.Edit(ctx, q,
			fmt.Sprintf("🪟 *%s* — not running anymore.", name),
			nil)
	}
	text, kb := keyboards.AppPanel(name, pid, data.Args[0])
	return r.Edit(ctx, q, text, kb)
}

func handleAppsQuit(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	name, ok := resolveAppName(d, data)
	if !ok {
		return errEdit(ctx, r, q, "🛑 *Quit*", fmt.Errorf("session expired — refresh the apps list"))
	}
	if !isConfirm(data.Args[1:]) {
		text, kb := keyboards.AppQuitConfirm(name, data.Args[0])
		return r.Edit(ctx, q, text, kb)
	}
	if err := d.Services.Apps.Quit(ctx, name); err != nil {
		return errEdit(ctx, r, q, fmt.Sprintf("🛑 *Quit %s* — failed", name), err)
	}
	return rerenderAppsListWithToast(ctx, r, q, d, fmt.Sprintf("🛑 Quit sent to *%s*.", name))
}

func handleAppsForce(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	name, ok := resolveAppName(d, data)
	if !ok {
		return errEdit(ctx, r, q, "💀 *Force Quit*", fmt.Errorf("session expired — refresh the apps list"))
	}
	pid, found, err := lookupAppPID(ctx, d, name)
	if err != nil {
		return errEdit(ctx, r, q, fmt.Sprintf("💀 *%s* — unavailable", name), err)
	}
	if !found {
		return r.Edit(ctx, q,
			fmt.Sprintf("💀 *%s* — not running anymore.", name), nil)
	}
	if !isConfirm(data.Args[1:]) {
		text, kb := keyboards.AppForceConfirm(name, pid, data.Args[0])
		return r.Edit(ctx, q, text, kb)
	}
	if err := d.Services.Apps.ForceQuit(ctx, pid); err != nil {
		return errEdit(ctx, r, q, fmt.Sprintf("💀 *Force Quit %s* — failed", name), err)
	}
	return rerenderAppsListWithToast(ctx, r, q, d,
		fmt.Sprintf("💀 SIGKILL sent to *%s* (PID `%d`).", name, pid))
}

func handleAppsHide(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	name, ok := resolveAppName(d, data)
	if !ok {
		return errEdit(ctx, r, q, "🙈 *Hide*", fmt.Errorf("session expired — refresh the apps list"))
	}
	if err := d.Services.Apps.Hide(ctx, name); err != nil {
		return errEdit(ctx, r, q, fmt.Sprintf("🙈 *Hide %s* — failed", name), err)
	}
	r.Toast(ctx, q, "Hidden.")
	// Re-render the per-app panel so the user can act again
	// without a full list round-trip. PID lookup post-hide is
	// best-effort — if the listing fails (TCC blip), fall back
	// to the bare list.
	pid, found, err := lookupAppPID(ctx, d, name)
	if err != nil || !found {
		return renderAppsList(ctx, r, q, d, 0)
	}
	text, kb := keyboards.AppPanel(name, pid, data.Args[0])
	return r.Edit(ctx, q, text, kb)
}

// resolveAppName resolves data.Args[0] (a [callbacks.ShortMap]
// id) back to the app name. Returns ("", false) when the args
// are empty or the ShortMap entry has expired (15-min TTL).
func resolveAppName(d *bot.Deps, data callbacks.Data) (string, bool) {
	if len(data.Args) < 1 {
		return "", false
	}
	return d.ShortMap.Get(data.Args[0])
}

// lookupAppPID re-fetches the running-app listing and returns
// the PID + found flag for name. A fresh listing on every call
// is the same approach [system.Service]'s findProc takes — we
// trade a re-shellout for a guarantee that the user sees
// post-quit / post-launch state.
//
// Returns (pid, true, nil) on hit, (0, false, nil) when the app
// isn't in the listing, or (0, false, err) on osascript error.
func lookupAppPID(ctx context.Context, d *bot.Deps, name string) (int, bool, error) {
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return 0, false, err
	}
	for _, a := range list {
		if a.Name == name {
			return a.PID, true, nil
		}
	}
	return 0, false, nil
}

// renderAppsList runs the listing, builds the per-row ShortMap
// items, and edits the message in place. Used by every "back to
// list" path on the dashboard.
func renderAppsList(ctx context.Context, r Reply, q *models.CallbackQuery, d *bot.Deps, page int) error {
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🪟 *Apps* — unavailable (Accessibility TCC?)", err)
	}
	items, totalPages := paginateApps(d, list, page)
	if page >= totalPages {
		page = 0
		items, totalPages = paginateApps(d, list, page)
	}
	text, kb := keyboards.AppsList(items, page, totalPages, len(list))
	return r.Edit(ctx, q, text, kb)
}

// paginateApps slices list to one page and stamps each item
// with a fresh ShortMap id.
func paginateApps(d *bot.Deps, list []apps.App, page int) ([]keyboards.AppListItem, int) {
	if len(list) == 0 {
		return nil, 1
	}
	totalPages := (len(list) + keyboards.AppsPageSize - 1) / keyboards.AppsPageSize
	start := page * keyboards.AppsPageSize
	if start >= len(list) {
		return nil, totalPages
	}
	end := start + keyboards.AppsPageSize
	if end > len(list) {
		end = len(list)
	}
	out := make([]keyboards.AppListItem, 0, end-start)
	for _, a := range list[start:end] {
		out = append(out, keyboards.AppListItem{
			Name:    a.Name,
			PID:     a.PID,
			Hidden:  a.Hidden,
			ShortID: d.ShortMap.Put(a.Name),
		})
	}
	return out, totalPages
}

// rerenderAppsListWithToast re-renders the list with a status
// banner stamped above it. Used after a successful destructive
// action so the user sees what happened in the same
// edit-in-place message.
func rerenderAppsListWithToast(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, msg string,
) error {
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🪟 *Apps* — unavailable", err)
	}
	items, totalPages := paginateApps(d, list, 0)
	text, kb := keyboards.AppsList(items, 0, totalPages, len(list))
	return r.Edit(ctx, q, msg+"\n\n"+text, kb)
}
