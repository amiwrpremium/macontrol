package handlers

import (
	"context"
	"strconv"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleSound is the Sound dashboard's callback dispatcher.
// Reached via the [callbacks.NSSound] namespace from any tap on
// the 🔊 Sound menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → run [sound.Service.Get], render the
//     dashboard via [keyboards.Sound]. Both share the same code
//     path; "refresh" is the user-tap entry, "open" is the
//     dispatched-from-home entry.
//  2. "up" / "down"      → adjust volume by ±delta via
//     [sound.Service.Adjust]. delta defaults to 5; data.Args[0]
//     overrides when present (parsed as int, silently
//     ignoring parse failures).
//  3. "max"              → run [sound.Service.Max] (volume to
//     100).
//  4. "mute" / "unmute"  → run the matching [sound.Service]
//     toggle method.
//  5. "set"              → install [flows.NewSetVolume] for a
//     typed exact value.
//
// Every successful action re-renders the dashboard via
// [keyboards.Sound] so the user sees the post-change state in
// the same edit-in-place message. Errors surface via [errEdit]
// with action-specific headers.
//
// Unknown actions fall through to a "Unknown sound action."
// toast.
func handleSound(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Sound
	switch data.Action {
	case "open", "refresh":
		r.Ack(ctx, q)
		st, err := svc.Get(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔊 *Sound* — unavailable", err)
		}
		text, kb := keyboards.Sound(st)
		return r.Edit(ctx, q, text, kb)

	case "up", "down":
		delta := 5
		if len(data.Args) > 0 {
			if v, err := strconv.Atoi(data.Args[0]); err == nil {
				delta = v
			}
		}
		if data.Action == "down" {
			delta = -delta
		}
		r.Ack(ctx, q)
		st, err := svc.Adjust(ctx, delta)
		if err != nil {
			return errEdit(ctx, r, q, "🔊 *Sound* — adjust failed", err)
		}
		text, kb := keyboards.Sound(st)
		return r.Edit(ctx, q, text, kb)

	case "max":
		r.Ack(ctx, q)
		st, err := svc.Max(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔊 *Sound* — max failed", err)
		}
		text, kb := keyboards.Sound(st)
		return r.Edit(ctx, q, text, kb)

	case "mute":
		r.Ack(ctx, q)
		st, err := svc.Mute(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔊 *Sound* — mute failed", err)
		}
		text, kb := keyboards.Sound(st)
		return r.Edit(ctx, q, text, kb)

	case "unmute":
		r.Ack(ctx, q)
		st, err := svc.Unmute(ctx)
		if err != nil {
			return errEdit(ctx, r, q, "🔊 *Sound* — unmute failed", err)
		}
		text, kb := keyboards.Sound(st)
		return r.Edit(ctx, q, text, kb)

	case "set":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSetVolume(svc)
		d.FlowReg.Install(chatID, f)
		resp := f.Start(ctx)
		return sendFlowPrompt(ctx, r, chatID, resp)
	}
	r.Toast(ctx, q, "Unknown sound action.")
	return nil
}
