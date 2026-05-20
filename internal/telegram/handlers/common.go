package handlers

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// errEdit replaces the callback's source message with a
// header + error line, using the standard "⚠ `<err>`" format.
//
// Behavior:
//   - Composes "<header>\n\n⚠ `<err>`" so the dashboard stays a
//     single edit-in-place message instead of spawning a
//     separate error reply.
//   - Passes nil for the keyboard so the buttons disappear —
//     the user has to navigate back via /menu. Trade-off:
//     simpler code at the cost of forcing a re-entry on
//     transient errors.
//
// Returns the [Reply.Edit] error verbatim. Used by every
// per-namespace handler's error branches.
func errEdit(ctx context.Context, r Reply, q *models.CallbackQuery, header string, err error) error {
	text := fmt.Sprintf("%s\n\n⚠ `%v`", header, err)
	return r.Edit(ctx, q, text, nil)
}

// isConfirm reports whether args starts with the "ok" sentinel
// used as the confirmation marker on destructive actions
// (force kill, shutdown, restart, logout).
//
// Behavior:
//   - Returns false on empty args.
//   - Returns args[0] == "ok" otherwise. Case-sensitive — the
//     keyboard layer always emits the lowercase literal.
//
// The destructive-action callbacks round-trip through a
// confirmation page that re-emits the original action with "ok"
// appended to args; this helper is the predicate the handler
// checks to decide "first tap → show confirm dialog" vs
// "second tap → actually do it".
func isConfirm(args []string) bool {
	return len(args) > 0 && args[0] == "ok"
}

// ClearLegacyReplyKB removes any persistent ReplyKeyboard from
// the chat by sending a one-character throwaway message with
// [models.ReplyKeyboardRemove], then deleting that message.
//
// Behavior:
//  1. Sends "·" with ReplyKeyboardRemove{RemoveKeyboard: true}.
//     Telegram's client processes the keyboard removal as soon
//     as the message arrives.
//  2. Deletes the message immediately. The keyboard removal
//     persists; only the visual "·" message disappears.
//  3. Errors at either step are swallowed — best-effort
//     cleanup that must never block the caller's user-visible
//     flow.
//
// Background: macontrol used a persistent ReplyKeyboard for
// the home menu before PR #22 (v0.1.4); users who upgraded see
// the stale buttons until something explicitly clears them.
// /start and /menu both call this defensively before sending
// the inline home grid so the upgrade is visible immediately.
func ClearLegacyReplyKB(ctx context.Context, d *bot.Deps, chatID int64) {
	msg, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "·",
		ReplyMarkup: &models.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	if err != nil || msg == nil {
		return
	}
	_, _ = d.Bot.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: msg.ID,
	})
}

// sendFlowPrompt sends the opening message of a newly-installed
// [flows.Flow]. Mirrors the dispatcher's per-step send logic in
// [bot.Deps.dispatchFlow]; used by handlers that install a flow
// in response to a callback (so the dispatcher hasn't seen the
// flow yet and can't emit the prompt itself).
//
// Behavior:
//   - When resp.Text is empty, returns nil — flows can produce
//     a "no opening prompt" Response when they want to do
//     something else first (rare).
//   - When resp.ParseMode is empty, defaults to
//     [models.ParseModeHTML] after running text through
//     [bot.MDToHTML] to convert any Markdown markers.
//   - Sends with the supplied resp.Markup as the reply
//     keyboard.
//
// Returns the upstream send error verbatim or nil on success.
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
