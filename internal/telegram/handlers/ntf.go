package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// handleNotify is the Notify dashboard's callback dispatcher.
// Reached via the [callbacks.NSNotify] namespace from any tap
// on the 🔔 Notify menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open" → render the Notify menu via [keyboards.Notify].
//  2. "send" → install [flows.NewSendNotify] for a typed
//     "title | body" desktop notification.
//  3. "say"  → install [flows.NewSay] for a typed
//     text-to-speech utterance.
//
// Both sub-actions are pure flow installs — there's no
// in-callback state to render. The flows themselves drive the
// user through prompt → input → result.
//
// Unknown actions fall through to a "Unknown notify action."
// toast.
func handleNotify(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Notify

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.Notify()
		return r.Edit(ctx, q, text, kb)

	case "send":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSendNotify(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))

	case "say":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewSay(svc)
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown notify action.")
	return nil
}
