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

// handleMedia is the Media dashboard's callback dispatcher.
// Reached via the [callbacks.NSMedia] namespace from any tap on
// the 📸 Media menu.
//
// Routing rules (data.Action — first match wins):
//  1. "open"   → render the Media menu via [keyboards.Media].
//  2. "shot"   → toast "Capturing…", run
//     [media.Service.Screenshot] with Silent gated on
//     data.Args[0] == "silent". On success uploads via
//     [Reply.SendPhoto] (which auto-deletes the temp file).
//     On failure sends a hint about the Screen Recording
//     TCC permission.
//  3. "photo"  → toast "Taking photo…", run
//     [media.Service.Photo] (webcam). Failure hint mentions
//     `brew install imagesnap` + Camera permission.
//  4. "record" → install [flows.NewRecord] for a typed
//     duration in seconds. The flow is wired up with
//     [newRecordSender] so it can upload the final .mov
//     without depending on Telegram types directly.
//
// Unknown actions fall through to a "Unknown media action."
// toast.
//
// Failure paths in shot/photo do NOT use [errEdit] (which
// would strip the keyboard) — they send a fresh message with
// the hint, leaving the Media menu intact for retries.
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

// newRecordSender adapts [Reply.SendVideo] into the
// dependency-injected sender shape that [flows.NewRecord]
// expects. Lets the Record flow upload the final .mov without
// needing to depend on Telegram types or the [bot.Deps]
// catalog directly.
//
// Behavior:
//   - Returns a closure that calls [Reply.SendVideo] with the
//     captured chatID and a fixed "📹 recording" caption.
//   - The closure's path argument is the temp .mov path the
//     Record flow returns from [media.Service.Record];
//     SendVideo deletes it after upload (success or failure).
func newRecordSender(r Reply, chatID int64) func(ctx context.Context, path string) error {
	return func(ctx context.Context, path string) error {
		return r.SendVideo(ctx, chatID, path, "📹 recording")
	}
}
