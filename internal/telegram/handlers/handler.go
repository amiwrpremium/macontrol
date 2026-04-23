package handlers

import (
	"context"
	"fmt"
	"os"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
)

// Reply wraps the common pattern of editing the message a callback came from
// (to update its state in place) with a fallback to SendMessage for message
// handlers.
type Reply struct {
	Deps *bot.Deps
}

// Send sends a new message to chatID. Markdown-style markers in text
// are rendered to HTML via bot.MDToHTML so Telegram's HTML parse mode
// accepts the output.
func (r Reply) Send(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) error {
	_, err := r.Deps.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        bot.MDToHTML(text),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return err
}

// Edit replaces the text + markup of the message backing the callback.
//
// A nil markup means "leave the keyboard untouched" — we deliberately
// omit the field rather than pass the typed-nil pointer through. The
// underlying ReplyMarkup field is `any`, and a typed-nil pointer
// assigned to an interface is non-nil (carries type info), defeating
// `omitempty` and producing `"reply_markup": null` — which Telegram
// rejects with "object expected as reply markup".
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

// Toast answers a callback query with an optional small toast.
func (r Reply) Toast(ctx context.Context, q *models.CallbackQuery, text string) {
	_, err := r.Deps.Bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: q.ID,
		Text:            text,
	})
	if err != nil {
		r.Deps.Logger.Debug("answerCallbackQuery", "err", err)
	}
}

// Ack answers a callback query without any toast (just clears the spinner).
func (r Reply) Ack(ctx context.Context, q *models.CallbackQuery) {
	r.Toast(ctx, q, "")
}

// SendPhoto uploads path as a photo to chatID, caption optional.
// The caller passes a path produced by our own domain/media package, so the
// file-inclusion warning on os.Open is accepted intentionally.
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

// SendVideo uploads path as a video to chatID. See SendPhoto for the
// trusted-path rationale.
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

// Code wraps s as a Markdown code block for safe multi-line output.
func Code(s string) string {
	return "```\n" + s + "\n```"
}
