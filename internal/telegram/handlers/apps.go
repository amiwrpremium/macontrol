package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	"open":         handleAppsOpen,
	"list":         handleAppsList,
	"show":         handleAppsShow,
	"quit":         handleAppsQuit,
	"force":        handleAppsForce,
	"hide":         handleAppsHide,
	"keep":         handleAppsKeep,
	"keep-toggle":  handleAppsKeepToggle,
	"keep-back":    handleAppsKeepBack,
	"keep-confirm": handleAppsKeepConfirm,
	"keep-execute": handleAppsKeepExecute,
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

// handleAppsKeep is the entry path for the "Quit all except…"
// flow. Reached from the Quit-all-except button on the apps
// list. Initialises an empty kept-set (= every app marked QUIT
// by default — matches the feature name) and renders the
// checklist.
func handleAppsKeep(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🚮 *Quit all except…* — unavailable", err)
	}
	sessionID := saveKeptSet(d, nil)
	return renderKeepChecklist(ctx, r, q, d, list, nil, sessionID)
}

// handleAppsKeepToggle flips one app's kept/quit state, re-stamps
// the session, and re-renders the checklist. Each tap burns one
// ShortMap entry; with the 15-min TTL and the existing janitor
// this is well within budget for any realistic toggle sequence.
func handleAppsKeepToggle(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	if len(data.Args) < 2 {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("missing session or app id"))
	}
	kept, ok := loadKeptSet(d, data.Args[0])
	if !ok {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("session expired — re-open the checklist"))
	}
	name, ok := d.ShortMap.Get(data.Args[1])
	if !ok {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("app reference expired — refresh the apps list"))
	}
	kept = toggleKept(kept, name)
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🚮 *Quit all except…* — unavailable", err)
	}
	sessionID := saveKeptSet(d, kept)
	return renderKeepChecklist(ctx, r, q, d, list, kept, sessionID)
}

// handleAppsKeepBack returns to the checklist from the confirm
// page with the same kept-set so the user can adjust the
// selection without starting over.
func handleAppsKeepBack(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	if len(data.Args) < 1 {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("missing session"))
	}
	kept, ok := loadKeptSet(d, data.Args[0])
	if !ok {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("session expired — re-open the checklist"))
	}
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🚮 *Quit all except…* — unavailable", err)
	}
	return renderKeepChecklist(ctx, r, q, d, list, kept, data.Args[0])
}

// handleAppsKeepConfirm renders the final "are you sure?" page
// with the explicit to-quit / to-keep lists. Computed against
// the current Running snapshot so a freshly-launched app shows
// up under "Will quit" too.
func handleAppsKeepConfirm(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	if len(data.Args) < 1 {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("missing session"))
	}
	kept, ok := loadKeptSet(d, data.Args[0])
	if !ok {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("session expired — re-open the checklist"))
	}
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🚮 *Quit all except…* — unavailable", err)
	}
	toQuit, toKeep := splitKeptList(list, kept)
	text, kb := keyboards.AppsKeepConfirm(toQuit, toKeep, data.Args[0])
	return r.Edit(ctx, q, text, kb)
}

// handleAppsKeepExecute runs Quit on every app in the to-quit
// set serially (not parallel) so any one failure doesn't
// cascade or race against the others, then re-renders the list
// with a summary banner.
//
// The "ok" sentinel is required (Confirm button stamps it) so
// stray traffic that arrives without confirmation can't trigger
// a bulk quit.
func handleAppsKeepExecute(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	if len(data.Args) < 2 || !isConfirm(data.Args[1:]) {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("missing confirmation"))
	}
	kept, ok := loadKeptSet(d, data.Args[0])
	if !ok {
		return errEdit(ctx, r, q, "🚮 *Quit all except…*", fmt.Errorf("session expired — re-open the checklist"))
	}
	list, err := d.Services.Apps.Running(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🚮 *Quit all except…* — unavailable", err)
	}
	toQuit, _ := splitKeptList(list, kept)
	failed := quitAll(ctx, d.Services.Apps, toQuit)
	banner := buildKeepExecuteBanner(toQuit, failed)
	return rerenderAppsListWithToast(ctx, r, q, d, banner)
}

// keptSet is the on-the-wire form of the multi-select selection
// — a JSON-marshalled []string of the names the user has
// marked KEEP. Stored in callbacks.ShortMap and addressed by a
// short opaque sessionID.
type keptSet = []string

// saveKeptSet marshals kept to JSON and parks it in the
// ShortMap, returning the freshly-issued sessionID. Burns one
// entry per call; tolerated by the 15-min ShortMap janitor.
func saveKeptSet(d *bot.Deps, kept keptSet) string {
	if kept == nil {
		kept = []string{}
	}
	raw, _ := json.Marshal(kept)
	return d.ShortMap.Put(string(raw))
}

// loadKeptSet resolves sessionID back to the kept-set. Returns
// (nil, false) when the entry has expired (15-min TTL).
func loadKeptSet(d *bot.Deps, sessionID string) (keptSet, bool) {
	raw, ok := d.ShortMap.Get(sessionID)
	if !ok {
		return nil, false
	}
	var kept keptSet
	if err := json.Unmarshal([]byte(raw), &kept); err != nil {
		return nil, false
	}
	return kept, true
}

// toggleKept adds name to kept when absent, removes it when
// present. Returns the (possibly newly-allocated) updated set.
func toggleKept(kept keptSet, name string) keptSet {
	for i, n := range kept {
		if n == name {
			return append(kept[:i], kept[i+1:]...)
		}
	}
	return append(kept, name)
}

// splitKeptList partitions the running app list into the
// to-quit and to-keep slices in alphabetical order. Names that
// are in kept but no longer in list are silently dropped (the
// app exited between toggle and confirm).
func splitKeptList(list []apps.App, kept keptSet) (toQuit, toKeep []string) {
	keep := map[string]bool{}
	for _, n := range kept {
		keep[n] = true
	}
	for _, a := range list {
		if keep[a.Name] {
			toKeep = append(toKeep, a.Name)
		} else {
			toQuit = append(toQuit, a.Name)
		}
	}
	sort.Strings(toQuit)
	sort.Strings(toKeep)
	return toQuit, toKeep
}

// quitAll runs Quit on every name serially. Returns the names
// for which the call returned an error so the caller can
// surface the partial-failure count.
func quitAll(ctx context.Context, svc *apps.Service, names []string) []string {
	failed := []string{}
	for _, n := range names {
		if err := svc.Quit(ctx, n); err != nil {
			failed = append(failed, n)
		}
	}
	return failed
}

// buildKeepExecuteBanner composes the status banner stamped
// above the post-execute list.
func buildKeepExecuteBanner(toQuit, failed []string) string {
	if len(toQuit) == 0 {
		return "🚮 *Quit all except…* — _nothing to quit._"
	}
	if len(failed) == 0 {
		return fmt.Sprintf("🚮 Sent quit to *%d* %s.", len(toQuit), nounApp(len(toQuit)))
	}
	return fmt.Sprintf("🚮 Sent quit to *%d* %s; *%d* failed.",
		len(toQuit)-len(failed), nounApp(len(toQuit)-len(failed)), len(failed))
}

// nounApp returns "app" or "apps" for English plural agreement.
func nounApp(n int) string {
	if n == 1 {
		return "app"
	}
	return "apps"
}

// renderKeepChecklist runs the listing, builds the per-row
// AppsKeepItem slice with each name's current kept state, and
// edits the message in place. Used by the entry path and every
// toggle.
func renderKeepChecklist(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, list []apps.App, kept keptSet, sessionID string,
) error {
	_ = ctx
	keep := map[string]bool{}
	for _, n := range kept {
		keep[n] = true
	}
	items := make([]keyboards.AppsKeepItem, 0, len(list))
	for _, a := range list {
		items = append(items, keyboards.AppsKeepItem{
			Name:    a.Name,
			Kept:    keep[a.Name],
			ShortID: d.ShortMap.Put(a.Name),
		})
	}
	text, kb := keyboards.AppsKeepChecklist(items, sessionID)
	return r.Edit(ctx, q, text, kb)
}
