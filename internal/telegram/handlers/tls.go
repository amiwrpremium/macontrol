package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleTools(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Tools

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.Tools(d.Capability.Features)
		return r.Edit(ctx, q, text, kb)

	case "clip":
		if len(data.Args) == 0 {
			r.Toast(ctx, q, "Missing sub-action.")
			return nil
		}
		switch data.Args[0] {
		case "get":
			r.Ack(ctx, q)
			text, err := svc.ClipboardRead(ctx)
			if err != nil {
				return errEdit(ctx, r, q, "📋 *Clipboard* — unavailable", err)
			}
			if len(text) > 3500 {
				text = text[:3500] + "\n…(truncated)"
			}
			body := "📋 *Clipboard*\n" + Code(text)
			_, kb := keyboards.Tools(d.Capability.Features)
			return r.Edit(ctx, q, body, kb)
		case "set":
			r.Ack(ctx, q)
			chatID := q.Message.Message.Chat.ID
			f := flows.NewClipSet(svc)
			d.FlowReg.Install(chatID, f)
			return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
		}

	case "tz":
		r.Ack(ctx, q)
		return renderTzRegions(ctx, r, q, d, svc)

	case "tz-region":
		r.Ack(ctx, q)
		if len(data.Args) == 0 {
			return errEdit(ctx, r, q, "🧭 *Timezone*", fmt.Errorf("missing region"))
		}
		return renderTzCities(ctx, r, q, d, svc, data.Args[0], 0, "")

	case "tz-page":
		r.Ack(ctx, q)
		if len(data.Args) == 0 {
			return errEdit(ctx, r, q, "🧭 *Timezone*", fmt.Errorf("missing region"))
		}
		region := data.Args[0]
		page, filterID := parseShortcutPageArgsAt(data, 1)
		filterTerm, _ := d.ShortMap.Get(filterID)
		return renderTzCities(ctx, r, q, d, svc, region, page, filterTerm)

	case "tz-set":
		if len(data.Args) == 0 {
			return errEdit(ctx, r, q, "🧭 *Timezone*", fmt.Errorf("missing timezone id"))
		}
		tz, ok := d.ShortMap.Get(data.Args[0])
		if !ok {
			return errEdit(ctx, r, q, "🧭 *Timezone*", fmt.Errorf("session expired — refresh the timezone list"))
		}
		r.Toast(ctx, q, fmt.Sprintf("Setting timezone → %s…", tz))
		var status string
		if err := svc.TimezoneSet(ctx, tz); err != nil {
			status = fmt.Sprintf("⚠ set failed: `%v`", err)
		} else {
			status = fmt.Sprintf("✅ Timezone set — `%s`", tz)
		}
		// Re-render the region picker with the status banner above.
		return rerenderTzRegionsWithStatus(ctx, r, q, d, svc, status)

	case "tz-search":
		if len(data.Args) == 0 {
			return errEdit(ctx, r, q, "🧭 *Timezone*", fmt.Errorf("missing region"))
		}
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewTimezoneSearch(svc, d.ShortMap, data.Args[0])
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "tz-type":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewTimezone(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "synctime":
		r.Toast(ctx, q, "Syncing clock…")
		if err := svc.TimeSync(ctx); err != nil {
			return errEdit(ctx, r, q, "🛠 *Tools* — sntp failed", err)
		}
		text, kb := keyboards.Tools(d.Capability.Features)
		return r.Edit(ctx, q, text+"\n\n_Clock synced._", kb)

	case "disks":
		r.Ack(ctx, q)
		vols, err := svc.DisksList(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "💿 *Disks* — unavailable", err)
		}
		rows := make([]keyboards.ToolsDiskRow, 0, len(vols))
		for _, v := range vols {
			rows = append(rows, keyboards.ToolsDiskRow{
				Mount:    v.MountedOn,
				Size:     v.Size,
				Capacity: v.Capacity,
				ShortID:  d.ShortMap.Put(v.MountedOn),
			})
		}
		body := "💿 *Disks*\n\nTap a disk for actions."
		if len(rows) == 0 {
			body = "💿 *Disks*\n\n_No user-facing volumes mounted._"
		}
		return r.Edit(ctx, q, body, keyboards.ToolsDisksList(rows))

	case "disk":
		r.Ack(ctx, q)
		mount, ok := resolveDiskMount(d, data)
		if !ok {
			return errEdit(ctx, r, q, "💿 *Disk*", fmt.Errorf("session expired — refresh the disks list"))
		}
		info, err := svc.DiskInfo(ctx, mount)
		if err != nil {
			return errEdit(ctx, r, q, fmt.Sprintf("💿 *%s* — diskutil failed", mount), err)
		}
		body := buildDiskPanel(mount, info)
		return r.Edit(ctx, q, body, keyboards.ToolsDiskPanel(data.Args[0], info.Removable))

	case "disk-open":
		mount, ok := resolveDiskMount(d, data)
		if !ok {
			return errEdit(ctx, r, q, "📂 *Open*", fmt.Errorf("session expired — refresh the disks list"))
		}
		if err := svc.OpenInFinder(ctx, mount); err != nil {
			return errEdit(ctx, r, q, fmt.Sprintf("📂 *Open %s* — failed", mount), err)
		}
		r.Toast(ctx, q, "Opened in Finder.")
		return nil

	case "disk-eject":
		mount, ok := resolveDiskMount(d, data)
		if !ok {
			return errEdit(ctx, r, q, "⏏ *Eject*", fmt.Errorf("session expired — refresh the disks list"))
		}
		if err := svc.EjectDisk(ctx, mount); err != nil {
			return errEdit(ctx, r, q, fmt.Sprintf("⏏ *Eject %s* — failed", mount), err)
		}
		r.Toast(ctx, q, "Ejected — re-rendering disks list.")
		// Re-fetch the list (the ejected disk should be gone).
		vols, _ := svc.DisksList(ctx)
		rows := make([]keyboards.ToolsDiskRow, 0, len(vols))
		for _, v := range vols {
			rows = append(rows, keyboards.ToolsDiskRow{
				Mount: v.MountedOn, Size: v.Size, Capacity: v.Capacity,
				ShortID: d.ShortMap.Put(v.MountedOn),
			})
		}
		return r.Edit(ctx, q, fmt.Sprintf("💿 *Disks*\n\n_Ejected `%s`._", mount),
			keyboards.ToolsDisksList(rows))

	case "shortcut":
		if !d.Capability.Features.Shortcuts {
			r.Toast(ctx, q, "Shortcuts CLI needs macOS 13+")
			return nil
		}
		r.Ack(ctx, q)
		return renderShortcutsPage(ctx, r, q, d, svc, 0, "")

	case "sc-page":
		r.Ack(ctx, q)
		page, filterID := parseShortcutPageArgs(data)
		filterTerm, _ := d.ShortMap.Get(filterID)
		return renderShortcutsPage(ctx, r, q, d, svc, page, filterTerm)

	case "sc-run":
		// args: <shortcutShortID> <page> <filterID>
		if len(data.Args) < 1 {
			return errEdit(ctx, r, q, "⚡ *Shortcut*", fmt.Errorf("missing shortcut id"))
		}
		name, ok := d.ShortMap.Get(data.Args[0])
		if !ok {
			return errEdit(ctx, r, q, "⚡ *Shortcut*", fmt.Errorf("session expired — refresh the list"))
		}
		r.Toast(ctx, q, fmt.Sprintf("▶ Running '%s'…", name))
		var status string
		if err := svc.ShortcutRun(ctx, name); err != nil {
			status = fmt.Sprintf("⚠ `%s` failed: `%v`", name, err)
		} else {
			status = fmt.Sprintf("✅ Ran `%s`.", name)
		}
		// Re-render at the same page+filter the user came from.
		page, filterID := parseShortcutPageArgsAt(data, 1)
		filterTerm, _ := d.ShortMap.Get(filterID)
		all, _ := svc.ShortcutsList(ctx)
		matches := flows.FilterShortcuts(all, filterTerm)
		items, totalPages := flows.PageShortcuts(matches, page, d.ShortMap)
		text, kb := keyboards.ToolsShortcutsList(items, page, totalPages, len(matches), filterID, filterTerm)
		return r.Edit(ctx, q, status+"\n\n"+text, kb)

	case "sc-search":
		if !d.Capability.Features.Shortcuts {
			r.Toast(ctx, q, "Shortcuts CLI needs macOS 13+")
			return nil
		}
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewShortcutSearch(svc, d.ShortMap)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "sc-type":
		if !d.Capability.Features.Shortcuts {
			r.Toast(ctx, q, "Shortcuts CLI needs macOS 13+")
			return nil
		}
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewShortcut(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown tools action.")
	return nil
}

// renderTzRegions edits the current message to the timezone region
// picker (step 1).
func renderTzRegions(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, svc *tools.Service,
) error {
	return renderTzRegionsWith(ctx, r, q, d, svc, "")
}

// rerenderTzRegionsWithStatus prepends a one-line status banner
// (success or failure of a tz-set) above the region picker.
func rerenderTzRegionsWithStatus(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, svc *tools.Service, status string,
) error {
	return renderTzRegionsWith(ctx, r, q, d, svc, status)
}

func renderTzRegionsWith(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, svc *tools.Service, status string,
) error {
	all, err := svc.TimezoneList(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🧭 *Timezone* — unavailable", err)
	}
	current, _ := svc.TimezoneCurrent(ctx)
	regions, topLevels := groupTimezones(all)
	regionRows := make([]keyboards.TimezoneRegion, 0, len(regions))
	for _, gr := range regions {
		regionRows = append(regionRows, keyboards.TimezoneRegion{
			Slug: gr.Slug, Count: len(gr.Tzs),
		})
	}
	topLevelRows := make([]keyboards.TimezoneTopLevel, 0, len(topLevels))
	for _, tz := range topLevels {
		topLevelRows = append(topLevelRows, keyboards.TimezoneTopLevel{
			Label:   tz,
			ShortID: d.ShortMap.Put(tz),
		})
	}
	text, kb := keyboards.ToolsTimezoneRegions(current, regionRows, topLevelRows)
	if status != "" {
		text = status + "\n\n" + text
	}
	return r.Edit(ctx, q, text, kb)
}

// renderTzCities edits the message to the city picker (step 2) for
// the given region, page, and optional filter substring.
func renderTzCities(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, svc *tools.Service, region string, page int, filterTerm string,
) error {
	all, err := svc.TimezoneList(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🧭 *Timezone* — unavailable", err)
	}
	current, _ := svc.TimezoneCurrent(ctx)
	cities := flows.FilterTimezonesInRegion(all, region, filterTerm)
	items, totalPages := flows.PageTimezones(cities, region, page, d.ShortMap)
	filterID := ""
	if filterTerm != "" {
		filterID = d.ShortMap.Put(filterTerm)
	}
	text, kb := keyboards.ToolsTimezoneCities(region, current, items, page, totalPages, len(cities), filterID, filterTerm)
	return r.Edit(ctx, q, text, kb)
}

// groupedRegion is a single region slug + its child timezones.
type groupedRegion struct {
	Slug string
	Tzs  []string
}

// groupTimezones splits the flat IANA timezone list into per-region
// buckets plus top-level (no '/') timezones. Region buckets are
// sorted alphabetically; cities within each region keep their
// original order from systemsetup.
func groupTimezones(all []string) (regions []groupedRegion, topLevels []string) {
	byRegion := map[string][]string{}
	regionOrder := []string{}
	for _, tz := range all {
		idx := strings.Index(tz, "/")
		if idx < 0 {
			topLevels = append(topLevels, tz)
			continue
		}
		region := tz[:idx]
		if _, seen := byRegion[region]; !seen {
			regionOrder = append(regionOrder, region)
		}
		byRegion[region] = append(byRegion[region], tz)
	}
	// Sort regions alphabetically for stable display.
	sortedRegions := append([]string(nil), regionOrder...)
	sort.Strings(sortedRegions)
	for _, slug := range sortedRegions {
		regions = append(regions, groupedRegion{Slug: slug, Tzs: byRegion[slug]})
	}
	return regions, topLevels
}

// renderShortcutsPage edits the current message to the Run-Shortcut
// list at the requested page + filter (filterTerm empty = unfiltered).
func renderShortcutsPage(ctx context.Context, r Reply, q *models.CallbackQuery,
	d *bot.Deps, svc *tools.Service, page int, filterTerm string,
) error {
	all, err := svc.ShortcutsList(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "⚡ *Run Shortcut* — unavailable", err)
	}
	matches := flows.FilterShortcuts(all, filterTerm)
	items, totalPages := flows.PageShortcuts(matches, page, d.ShortMap)
	filterID := ""
	if filterTerm != "" {
		filterID = d.ShortMap.Put(filterTerm)
	}
	text, kb := keyboards.ToolsShortcutsList(items, page, totalPages, len(matches), filterID, filterTerm)
	return r.Edit(ctx, q, text, kb)
}

// parseShortcutPageArgs extracts (page, filterID) from a sc-page
// callback's args. filterID is "" when the callback used the "-"
// sentinel for unfiltered.
func parseShortcutPageArgs(data callbacks.Data) (page int, filterID string) {
	return parseShortcutPageArgsAt(data, 0)
}

// parseShortcutPageArgsAt extracts (page, filterID) from data.Args
// starting at offset. Used by sc-run, which carries its own arg
// before the page+filter pair.
func parseShortcutPageArgsAt(data callbacks.Data, offset int) (page int, filterID string) {
	if len(data.Args) > offset {
		page, _ = strconv.Atoi(data.Args[offset])
		if page < 0 {
			page = 0
		}
	}
	if len(data.Args) > offset+1 {
		filterID = data.Args[offset+1]
		if filterID == "-" {
			filterID = ""
		}
	}
	return page, filterID
}

// resolveDiskMount looks up data.Args[0] (a ShortMap id) and returns
// the original mount path. Returns ok=false if the id is missing or
// expired (15-min TTL — user kept a stale dashboard open).
func resolveDiskMount(d *bot.Deps, data callbacks.Data) (string, bool) {
	if len(data.Args) == 0 {
		return "", false
	}
	return d.ShortMap.Get(data.Args[0])
}

// buildDiskPanel renders the per-disk drill-down body. Falls back to
// a minimal text + raw diskutil block if parsing didn't capture the
// labelled fields.
func buildDiskPanel(mount string, info tools.DiskDetails) string {
	var b strings.Builder
	name := info.VolumeName
	if name == "" {
		name = mount
	}
	fmt.Fprintf(&b, "💿 *%s*", name)
	if info.DiskSize != "" {
		fmt.Fprintf(&b, " — `%s` total", info.DiskSize)
	}
	b.WriteString("\n")

	if info.UsedSpace != "" || info.FreeSpace != "" {
		fmt.Fprintf(&b, "Used: `%s` · Free: `%s`\n", nonEmpty(info.UsedSpace), nonEmpty(info.FreeSpace))
	}
	if info.FSType != "" || info.Device != "" {
		fmt.Fprintf(&b, "FS: `%s` · Device: `%s`\n", nonEmpty(info.FSType), nonEmpty(info.Device))
	}
	location := "Internal"
	if !info.Internal {
		location = "External"
	}
	media := "Fixed"
	if info.Removable {
		media = "Removable"
	}
	storage := ""
	if info.SolidState {
		storage = " · SSD"
	}
	fmt.Fprintf(&b, "_%s · %s%s_", location, media, storage)
	if info.ReadOnly {
		b.WriteString(" · _read-only_")
	}
	if info.VolumeName == "" && info.DiskSize == "" {
		// Parser saw nothing useful — surface the raw diskutil text.
		b.WriteString("\n\n" + Code(truncate(info.Raw, 1500)))
	}
	return b.String()
}

func nonEmpty(s string) string {
	if s == "" {
		return "?"
	}
	return s
}
