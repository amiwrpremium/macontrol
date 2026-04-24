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
// soundDispatch maps Sound callback actions to per-action handlers.
// "open" and "refresh" share a handler; "up" and "down" share one
// with a sign-flip inside.
var soundDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":    handleSoundRefresh,
	"refresh": handleSoundRefresh,
	"up":      handleSoundNudge,
	"down":    handleSoundNudge,
	"max":     handleSoundMax,
	"mute":    handleSoundMute,
	"unmute":  handleSoundUnmute,
	"set":     handleSoundSet,
}

func handleSound(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := soundDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown sound action.")
		return nil
	}
	return h(ctx, d, q, data)
}

func handleSoundRefresh(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Sound.Get(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔊 *Sound* — unavailable", err)
	}
	text, kb := keyboards.Sound(st)
	return r.Edit(ctx, q, text, kb)
}

func handleSoundNudge(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
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
	st, err := d.Services.Sound.Adjust(ctx, delta)
	if err != nil {
		return errEdit(ctx, r, q, "🔊 *Sound* — adjust failed", err)
	}
	text, kb := keyboards.Sound(st)
	return r.Edit(ctx, q, text, kb)
}

func handleSoundMax(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Sound.Max(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔊 *Sound* — max failed", err)
	}
	text, kb := keyboards.Sound(st)
	return r.Edit(ctx, q, text, kb)
}

func handleSoundMute(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Sound.Mute(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔊 *Sound* — mute failed", err)
	}
	text, kb := keyboards.Sound(st)
	return r.Edit(ctx, q, text, kb)
}

func handleSoundUnmute(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	st, err := d.Services.Sound.Unmute(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🔊 *Sound* — unmute failed", err)
	}
	text, kb := keyboards.Sound(st)
	return r.Edit(ctx, q, text, kb)
}

func handleSoundSet(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	chatID := q.Message.Message.Chat.ID
	f := flows.NewSetVolume(d.Services.Sound)
	d.FlowReg.Install(chatID, f)
	return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
}
