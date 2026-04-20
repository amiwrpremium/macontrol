package handlers

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// errEdit replaces a callback's message with an error line. Used when the
// underlying domain call fails — keeps the dashboard tidy instead of
// spawning a separate error message.
func errEdit(ctx context.Context, r Reply, q *models.CallbackQuery, header string, err error) error {
	text := fmt.Sprintf("%s\n\n⚠ `%v`", header, err)
	return r.Edit(ctx, q, text, nil)
}

// sendFlowPrompt sends the first message of a newly-installed flow.
func sendFlowPrompt(ctx context.Context, r Reply, chatID int64, resp flows.Response) error {
	if resp.Text == "" {
		return nil
	}
	parseMode := resp.ParseMode
	if parseMode == "" {
		parseMode = models.ParseModeMarkdown
	}
	_, err := r.Deps.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        resp.Text,
		ParseMode:   parseMode,
		ReplyMarkup: resp.Markup,
	})
	return err
}
