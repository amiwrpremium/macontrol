// Package telegramtest provides an httptest-based fake Telegram Bot API
// server that records every outgoing request. It's the standard harness
// the handler/command/bot tests use — no production code changes required.
//
// Construct with NewBot(t); it returns a real *tgbot.Bot pointed at an
// in-process httptest.Server plus a Recorder that captures each call.
package telegramtest

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Call is one captured Telegram API request.
type Call struct {
	// Method is the trailing API verb — "sendMessage", "editMessageText",
	// "answerCallbackQuery", "sendPhoto", "sendVideo", etc.
	Method string
	// Fields captures multipart/form-data values (non-file fields).
	Fields map[string]string
	// Files captures uploaded filenames keyed by form field name.
	Files map[string]string
}

// Recorder accumulates Calls.
type Recorder struct {
	mu    sync.Mutex
	calls []Call
	// Response is used to reply to every API call. Defaults to
	// `{"ok":true,"result":{"message_id":1}}` which keeps almost every
	// library code-path happy; override for error-path tests.
	Response []byte
}

// Calls returns a snapshot of recorded requests.
func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Call, len(r.calls))
	copy(out, r.calls)
	return out
}

// Last returns the most recent call or panics if there are none.
func (r *Recorder) Last() Call {
	c := r.Calls()
	if len(c) == 0 {
		panic("telegramtest: no calls recorded")
	}
	return c[len(c)-1]
}

// ByMethod filters Calls to those matching a specific API verb.
func (r *Recorder) ByMethod(method string) []Call {
	out := []Call{}
	for _, c := range r.Calls() {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// Reset clears recorded calls.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

// NewBot returns a real *tgbot.Bot wired to an in-process Telegram API
// stub. The returned cleanup has already been registered with t.Cleanup.
func NewBot(t *testing.T) (*tgbot.Bot, *Recorder) {
	t.Helper()
	rec := &Recorder{Response: defaultOKResponse}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		call := parseCall(req)
		rec.mu.Lock()
		rec.calls = append(rec.calls, call)
		resp := rec.Response
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	b, err := tgbot.New("test-token-12345:AAA",
		tgbot.WithServerURL(srv.URL),
		tgbot.WithSkipGetMe(),
	)
	if err != nil {
		t.Fatalf("tgbot.New: %v", err)
	}
	return b, rec
}

// parseCall decodes one incoming request into a Call.
func parseCall(req *http.Request) Call {
	call := Call{Fields: map[string]string{}, Files: map[string]string{}}

	// URL is /bot<token>/<method>
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
	if len(parts) >= 2 {
		call.Method = parts[1]
	}

	ct, params, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if ct == "multipart/form-data" {
		if err := req.ParseMultipartForm(8 << 20); err == nil {
			for k, v := range req.MultipartForm.Value {
				if len(v) > 0 {
					call.Fields[k] = v[0]
				}
			}
			for k, files := range req.MultipartForm.File {
				if len(files) > 0 {
					call.Files[k] = files[0].Filename
				}
			}
		}
	}
	_ = params
	return call
}

// MustDecodeInlineKeyboard extracts the reply_markup field from a Call,
// unmarshals it as InlineKeyboardMarkup, and fails the test on error.
func MustDecodeInlineKeyboard(t *testing.T, c Call) *models.InlineKeyboardMarkup {
	t.Helper()
	raw, ok := c.Fields["reply_markup"]
	if !ok {
		t.Fatalf("call %s has no reply_markup field (fields=%v)", c.Method, c.Fields)
	}
	var kb models.InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(raw), &kb); err != nil {
		t.Fatalf("decode reply_markup: %v (raw=%s)", err, raw)
	}
	return &kb
}

// MustDecodeReplyKeyboard is the ReplyKeyboardMarkup counterpart.
func MustDecodeReplyKeyboard(t *testing.T, c Call) *models.ReplyKeyboardMarkup {
	t.Helper()
	raw, ok := c.Fields["reply_markup"]
	if !ok {
		t.Fatalf("call %s has no reply_markup field", c.Method)
	}
	var kb models.ReplyKeyboardMarkup
	if err := json.Unmarshal([]byte(raw), &kb); err != nil {
		t.Fatalf("decode reply_markup: %v (raw=%s)", err, raw)
	}
	return &kb
}

var defaultOKResponse = []byte(`{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}}`)
