package handlers

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// errEdit replaces a callback's message with an error line. Used when the
// underlying domain call fails — keeps the dashboard tidy instead of
// spawning a separate error message.
func errEdit(ctx context.Context, r Reply, q *models.CallbackQuery, header string, err error) error {
	text := fmt.Sprintf("%s\n\n⚠ `%v`", header, err)
	return r.Edit(ctx, q, text, nil)
}

// isConfirm reports whether the args slice starts with the "ok"
// confirmation marker. Used by destructive actions (power off, force
// kill) that round-trip through a confirmation page.
func isConfirm(args []string) bool {
	return len(args) > 0 && args[0] == "ok"
}

// sendFlowPrompt sends the first message of a newly-installed flow.
func sendFlowPrompt(ctx context.Context, r Reply, chatID int64, resp flows.Response) error {
	if resp.Text == "" {
		return nil
	}
	parseMode := resp.ParseMode
	text := resp.Text
	if parseMode == "" {
		parseMode = models.ParseModeHTML
		text = bot.MDToHTML(text)
	}
	_, err := r.Deps.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   parseMode,
		ReplyMarkup: resp.Markup,
	})
	return err
}
