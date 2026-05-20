package handlers_test

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

// Edit with a nil markup must omit the reply_markup field entirely.
//
// Regression test for the typed-nil interface gotcha: the bot library's
// EditMessageTextParams.ReplyMarkup is `any`, so a typed-nil
// `*InlineKeyboardMarkup` becomes a non-nil interface and JSON-marshals
// to `"reply_markup": null`. Telegram rejects that with
// "object expected as reply markup", which used to crash every errEdit
// call (see internal/telegram/handlers/common.go).
func TestEdit_NilMarkupOmitsField(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := handlers.Reply{Deps: h.Deps}
	q := newCallbackUpdate("cb-1", "noop").CallbackQuery

	if err := r.Edit(context.Background(), q, "hello", nil); err != nil {
		t.Fatalf("Edit returned error: %v", err)
	}

	calls := h.Recorder.ByMethod("editMessageText")
	if len(calls) != 1 {
		t.Fatalf("want 1 editMessageText call, got %d (%v)", len(calls), h.Recorder.Calls())
	}
	if raw, present := calls[0].Fields["reply_markup"]; present {
		t.Fatalf("reply_markup must be omitted when markup is nil, got %q", raw)
	}
}

// Edit with a real markup must serialize it as a JSON object Telegram
// will accept.
func TestEdit_NonNilMarkupSerialized(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := handlers.Reply{Deps: h.Deps}
	q := newCallbackUpdate("cb-2", "noop").CallbackQuery

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "x", CallbackData: "y"}}},
	}
	if err := r.Edit(context.Background(), q, "hello", kb); err != nil {
		t.Fatalf("Edit returned error: %v", err)
	}

	calls := h.Recorder.ByMethod("editMessageText")
	if len(calls) != 1 {
		t.Fatalf("want 1 editMessageText call, got %d", len(calls))
	}
	if _, present := calls[0].Fields["reply_markup"]; !present {
		t.Fatal("reply_markup should be present when markup is non-nil")
	}
}
