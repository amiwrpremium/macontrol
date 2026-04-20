package handlers

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/media"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func handleMedia(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	svc := d.Services.Media

	switch data.Action {
	case "open":
		r.Ack(ctx, q)
		text, kb := keyboards.Media()
		return r.Edit(ctx, q, text, kb)

	case "shot":
		silent := len(data.Args) > 0 && data.Args[0] == "silent"
		r.Toast(ctx, q, "📷 Capturing…")
		chatID := q.Message.Message.Chat.ID
		path, err := svc.Screenshot(ctx, media.ScreenshotOpts{Silent: silent})
		if err != nil {
			return r.Send(ctx, chatID, "⚠ screenshot failed: `"+err.Error()+"`\n\n_Did you grant Screen Recording in System Settings?_", nil)
		}
		return r.SendPhoto(ctx, chatID, path, "📷 screenshot")

	case "photo":
		r.Toast(ctx, q, "📸 Taking photo…")
		chatID := q.Message.Message.Chat.ID
		path, err := svc.Photo(ctx)
		if err != nil {
			return r.Send(ctx, chatID, "⚠ webcam failed: `"+err.Error()+"`\n\n_Install `brew install imagesnap` and grant Camera permission._", nil)
		}
		return r.SendPhoto(ctx, chatID, path, "📸 webcam")

	case "record":
		r.Ack(ctx, q)
		chatID := q.Message.Message.Chat.ID
		f := flows.NewRecord(svc, chatID, newRecordSender(r, chatID))
		d.FlowReg.Install(chatID, f)
		return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
	}
	r.Toast(ctx, q, "Unknown media action.")
	return nil
}

// newRecordSender adapts the Reply helper for the Record flow — so the flow
// can send the final video without knowing about Telegram types.
func newRecordSender(r Reply, chatID int64) func(ctx context.Context, path string) error {
	return func(ctx context.Context, path string) error {
		return r.SendVideo(ctx, chatID, path, "📹 recording")
	}
}
