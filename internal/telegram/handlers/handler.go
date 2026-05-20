package handlers

import (
	"context"
	"fmt"
	"os"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
)

// Reply is the per-handler convenience wrapper around the upstream
// bot client. Every per-namespace handler receives a freshly
// constructed Reply (literally `Reply{Deps: d}`) and uses it for
// every outbound Telegram operation: sending, editing, toasting,
// and uploading photos/videos. Centralising these helpers keeps the
// handlers free of `bot.Bot.*` boilerplate and lets us apply
// consistent parse-mode, markup-nil, and tempfile-cleanup policies.
//
// Lifecycle:
//   - One Reply per handler invocation. Stateless — the only field
//     is a pointer to [bot.Deps], which itself is process-wide.
//   - Methods may be called any number of times; each call is
//     independent.
//
// Concurrency:
//   - Safe to copy and use from any goroutine; Reply holds only a
//     pointer to the (concurrent-safe) [bot.Deps].
//
// Field roles:
//   - Deps is the wired-up dependency bag — Bot, Logger,
//     Whitelist, Services, etc. See [bot.Deps].
type Reply struct {
	// Deps is the dependency bag every method dereferences for
	// the bot client and the logger. Never nil during normal
	// operation; methods will panic on nil.
	Deps *bot.Deps
}

// Send delivers a brand-new message to chatID.
//
// Behavior:
//   - Always sends with [models.ParseModeHTML] so any inline
//     formatting in text is parsed by Telegram.
//   - Runs text through [bot.MDToHTML] to convert the
//     Markdown-style markers our handlers compose by reflex
//     (`*bold*`, `_italic_`, “code“) into the HTML the
//     parse mode actually expects.
//   - markup may be any of the InlineKeyboardMarkup,
//     ReplyKeyboardMarkup, ReplyKeyboardRemove, ForceReply
//     concrete types — the upstream library treats it as `any`.
//
// Returns the upstream send error verbatim or nil on success.
func (r Reply) Send(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) error {
	_, err := r.Deps.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        bot.MDToHTML(text),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return err
}

// Edit replaces the text and (optionally) the inline keyboard of
// the message that produced callback q. This is the workhorse
// for stateful dashboards: every "+1 volume" tap, every "Refresh"
// tap, every drill-down ends up here so the user's chat stays a
// single edit-in-place message rather than an ever-growing
// conversation.
//
// Behavior:
//   - Reads the source message off q.Message.Message; returns an
//     error if the message is inaccessible (e.g. a callback fired
//     against a deleted message).
//   - Renders text via [bot.MDToHTML] and sends with
//     [models.ParseModeHTML], same as [Reply.Send].
//   - When markup is nil, deliberately omits the ReplyMarkup field
//     entirely rather than passing the typed-nil pointer through.
//     Reason: ReplyMarkup is `any`, and a typed-nil
//     `*InlineKeyboardMarkup` assigned to an interface is
//     non-nil-with-type-info, so the json encoder defeats
//     `omitempty` and emits `"reply_markup": null`. Telegram
//     rejects that with "object expected as reply markup". This
//     bug is what motivated PR #31 — see internal/telegram/bot
//     git history.
//
// Returns the upstream edit error verbatim, or nil on success.
func (r Reply) Edit(ctx context.Context, q *models.CallbackQuery, text string, markup *models.InlineKeyboardMarkup) error {
	msg := q.Message.Message
	if msg == nil {
		return fmt.Errorf("callback message is not accessible")
	}
	params := &tgbot.EditMessageTextParams{
		ChatID:    msg.Chat.ID,
		MessageID: msg.ID,
		Text:      bot.MDToHTML(text),
		ParseMode: models.ParseModeHTML,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, err := r.Deps.Bot.EditMessageText(ctx, params)
	return err
}

// Toast answers a callback query with a small ephemeral popup
// over the chat ("Speedtest needs macOS 12+", "Running — takes
// ~15s…"). Telegram clears the spinner the tap triggered and
// shows text for ~3s.
//
// Behavior:
//   - Returns nothing; any upstream answerCallbackQuery error is
//     logged at DEBUG and otherwise swallowed because the user
//     has already seen *something* happen on tap, and a noisy
//     WARN here would clutter the logs.
//
// Pass an empty string for text to clear the spinner without
// showing a popup; [Reply.Ack] is the named convenience for that.
func (r Reply) Toast(ctx context.Context, q *models.CallbackQuery, text string) {
	_, err := r.Deps.Bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: q.ID,
		Text:            text,
	})
	if err != nil {
		r.Deps.Logger.Debug("answerCallbackQuery", "err", err)
	}
}

// Ack answers a callback query with no popup — clears Telegram's
// spinner and nothing more. Equivalent to `Toast(ctx, q, "")`.
// Use it whenever the handler is about to call Edit; the spinner
// must be answered within ~5s or the client shows a stale "tap
// rejected" indicator.
func (r Reply) Ack(ctx context.Context, q *models.CallbackQuery) {
	r.Toast(ctx, q, "")
}

// SendPhoto uploads path as a photo to chatID with an optional
// caption.
//
// Behavior:
//   - The file at path is opened, streamed up via the upstream
//     SendPhoto, and **deleted on return** regardless of success
//     (best-effort; remove errors are silently ignored).
//   - The Telegram-side filename is hard-coded to "screenshot.png"
//     because every caller in macontrol is the screenshot
//     pipeline. If new callers appear with different content,
//     parameterise.
//   - The `#nosec G304` is intentional: callers always supply a
//     path produced by [internal/domain/media] (a private tempdir
//     under os.TempDir), never user-controlled.
//
// Returns "open photo: <err>" wrapping the os.Open failure, or
// the upstream SendPhoto error, or nil on success.
func (r Reply) SendPhoto(ctx context.Context, chatID int64, path, caption string) error {
	defer func() { _ = os.Remove(path) }()
	f, err := os.Open(path) // #nosec G304 -- trusted caller-supplied tempfile
	if err != nil {
		return fmt.Errorf("open photo: %w", err)
	}
	defer func() { _ = f.Close() }()
	_, err = r.Deps.Bot.SendPhoto(ctx, &tgbot.SendPhotoParams{
		ChatID:  chatID,
		Photo:   &models.InputFileUpload{Filename: "screenshot.png", Data: f},
		Caption: caption,
	})
	return err
}

// SendVideo uploads path as a video to chatID with an optional
// caption. Same lifecycle and trust contract as [Reply.SendPhoto]
// — the file is opened, streamed up, and deleted on return; the
// Telegram-side filename is hard-coded to "recording.mov" since
// every caller is the screen-record pipeline.
//
// Returns "open video: <err>" wrapping the os.Open failure, or
// the upstream SendVideo error, or nil on success.
func (r Reply) SendVideo(ctx context.Context, chatID int64, path, caption string) error {
	defer func() { _ = os.Remove(path) }()
	f, err := os.Open(path) // #nosec G304 -- trusted caller-supplied tempfile
	if err != nil {
		return fmt.Errorf("open video: %w", err)
	}
	defer func() { _ = f.Close() }()
	_, err = r.Deps.Bot.SendVideo(ctx, &tgbot.SendVideoParams{
		ChatID:  chatID,
		Video:   &models.InputFileUpload{Filename: "recording.mov", Data: f},
		Caption: caption,
	})
	return err
}

// Code wraps s in a fenced Markdown code block. Used by handlers
// that want to render multi-line CLI output (wdutil dump,
// system_profiler dump, brightness error text) without Telegram's
// HTML parser eating any angle-brackets or ampersands.
//
// The returned string is intended to flow through [bot.MDToHTML]
// (via [Reply.Send] / [Reply.Edit]), which converts the fence to
// the equivalent <pre><code>…</code></pre> block.
func Code(s string) string {
	return "```\n" + s + "\n```"
}
