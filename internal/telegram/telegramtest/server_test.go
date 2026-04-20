package telegramtest_test

import (
	"context"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/telegramtest"
)

func TestNewBot_RecordsSendMessage(t *testing.T) {
	t.Parallel()
	b, rec := telegramtest.NewBot(t)

	_, err := b.SendMessage(context.Background(), &tgbot.SendMessageParams{
		ChatID: int64(123),
		Text:   "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := rec.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].Method != "sendMessage" {
		t.Fatalf("method = %q", calls[0].Method)
	}
	if calls[0].Fields["chat_id"] != "123" {
		t.Fatalf("chat_id = %q", calls[0].Fields["chat_id"])
	}
	if calls[0].Fields["text"] != "hello" {
		t.Fatalf("text = %q", calls[0].Fields["text"])
	}
}

func TestRecorder_ByMethodFiltering(t *testing.T) {
	t.Parallel()
	b, rec := telegramtest.NewBot(t)

	_, _ = b.SendMessage(context.Background(), &tgbot.SendMessageParams{ChatID: 1, Text: "a"})
	_, _ = b.EditMessageText(context.Background(), &tgbot.EditMessageTextParams{
		ChatID: 1, MessageID: 1, Text: "b",
	})
	if got := len(rec.ByMethod("sendMessage")); got != 1 {
		t.Errorf("sendMessage count = %d", got)
	}
	if got := len(rec.ByMethod("editMessageText")); got != 1 {
		t.Errorf("editMessageText count = %d", got)
	}
	if got := len(rec.ByMethod("nope")); got != 0 {
		t.Errorf("nope count = %d", got)
	}
}

func TestRecorder_Reset(t *testing.T) {
	t.Parallel()
	b, rec := telegramtest.NewBot(t)
	_, _ = b.SendMessage(context.Background(), &tgbot.SendMessageParams{ChatID: 1, Text: "x"})
	if len(rec.Calls()) != 1 {
		t.Fatal("expected 1 call")
	}
	rec.Reset()
	if len(rec.Calls()) != 0 {
		t.Fatal("expected 0 after reset")
	}
}

func TestRecorder_LastPanicsWhenEmpty(t *testing.T) {
	t.Parallel()
	_, rec := telegramtest.NewBot(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = rec.Last()
}

func TestMustDecodeInlineKeyboard(t *testing.T) {
	t.Parallel()
	b, rec := telegramtest.NewBot(t)
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "A", CallbackData: "ns:act"}},
		},
	}
	_, err := b.SendMessage(context.Background(), &tgbot.SendMessageParams{
		ChatID: 1, Text: "x", ReplyMarkup: kb,
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded := telegramtest.MustDecodeInlineKeyboard(t, rec.Last())
	if len(decoded.InlineKeyboard) != 1 || decoded.InlineKeyboard[0][0].Text != "A" {
		t.Fatalf("unexpected decoded kb: %+v", decoded)
	}
}
