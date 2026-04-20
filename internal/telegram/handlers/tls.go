package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"

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
		var b strings.Builder
		fmt.Fprintf(&b, "%-12s %-8s %-8s %-4s %s\n", "FS", "Size", "Used", "Cap", "Mount")
		for _, v := range vols {
			fmt.Fprintf(&b, "%-12s %-8s %-8s %-4s %s\n", v.Filesystem, v.Size, v.Used, v.Capacity, v.MountedOn)
		}
		_, kb := keyboards.Tools(d.Capability.Features)
		return r.Edit(ctx, q, "💿 *Disks*\n"+Code(b.String()), kb)

	case "shortcut":
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
