// Package telegramtest provides an httptest-based fake Telegram Bot
// API server that records every outgoing request. It's the standard
// harness the handler/command/bot tests use — no production code
// changes required.
//
// Construct with [NewBot] inside a test; it returns a real
// *tgbot.Bot pointed at an in-process httptest.Server plus a
// [Recorder] that captures each outbound call. The cleanup is wired
// to t.Cleanup so the test doesn't have to release the server
// manually.
//
// The package intentionally lives outside the main test build path
// (no _test suffix) so test helpers can be imported across packages.
// It is NOT for production use.
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

// Call is one captured Telegram API request. Built by [parseCall]
// inside the httptest handler each time the bot library makes an
// outbound HTTP call.
//
// Field roles:
//   - Method is the trailing API verb extracted from the request
//     URL — e.g. "sendMessage", "editMessageText",
//     "answerCallbackQuery", "sendPhoto", "sendVideo".
//   - Fields captures multipart/form-data values (non-file fields).
//     Single-valued; if the same key appears twice, only the first
//     value survives.
//   - Files captures uploaded filenames keyed by form field name.
//     Same single-value semantics as Fields. Only the filename is
//     captured — file contents are NOT preserved.
type Call struct {
	// Method is the trailing API verb — "sendMessage",
	// "editMessageText", "answerCallbackQuery", "sendPhoto",
	// "sendVideo", etc.
	Method string

	// Fields captures multipart/form-data values (non-file fields).
	Fields map[string]string

	// Files captures uploaded filenames keyed by form field name.
	Files map[string]string
}

// Recorder accumulates [Call]s as the bot library makes API
// requests. Each [NewBot] returns its own Recorder; recorders are
// not shared across tests.
//
// Concurrency:
//   - All public methods take mu so a recorder is safe to access
//     from the bot's goroutines and the test goroutine
//     concurrently.
//
// Field roles:
//   - mu serialises every access to calls and Response.
//   - calls is the append-only call log.
//   - Response is the canned JSON body returned for every API call.
//     Defaults to a successful sendMessage-shaped reply that keeps
//     almost every library code-path happy. Override per-test for
//     error-path coverage.
type Recorder struct {
	mu    sync.Mutex
	calls []Call

	// Response is used to reply to every API call. Defaults to
	// `{"ok":true,"result":{"message_id":1,...}}` which keeps
	// almost every library code-path happy; override for
	// error-path tests.
	Response []byte
}

// Calls returns a snapshot copy of the recorded requests. Safe to
// inspect without holding the recorder mutex; the slice is owned
// by the caller.
func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Call, len(r.calls))
	copy(out, r.calls)
	return out
}

// Last returns the most recent call.
//
// Panics with "telegramtest: no calls recorded" when the recorder
// is empty — tests that may legitimately have zero calls should
// check len([Recorder.Calls]()) first.
func (r *Recorder) Last() Call {
	c := r.Calls()
	if len(c) == 0 {
		panic("telegramtest: no calls recorded")
	}
	return c[len(c)-1]
}

// ByMethod returns the subset of recorded calls whose Method
// matches the given API verb. Returns an empty (non-nil) slice
// when no calls match.
//
// Useful for asserting "there was exactly one sendPhoto call" by
// pairing with `len(rec.ByMethod("sendPhoto"))`.
func (r *Recorder) ByMethod(method string) []Call {
	out := []Call{}
	for _, c := range r.Calls() {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// Reset clears recorded calls. Use between sub-cases of a single
// test that exercises multiple flows against the same Recorder.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

// NewBot returns a real *tgbot.Bot wired to an in-process Telegram
// API stub plus the [Recorder] that captures every outbound
// request.
//
// Behavior:
//   - Spins up an [httptest.NewServer] whose handler decodes each
//     request into a [Call], appends to the recorder, and replies
//     with the recorder's Response field (default OK shape).
//   - Constructs the *tgbot.Bot with [tgbot.WithServerURL] pointing
//     at the test server and [tgbot.WithSkipGetMe] so the library
//     doesn't issue a startup self-check.
//   - Registers srv.Close with t.Cleanup so the server is torn
//     down with the test.
//   - Calls t.Fatalf on tgbot.New error — there is no graceful
//     fallback path; a failed bot construction is a test bug.
//
// The dummy token "test-token-12345:AAA" is not validated by the
// fake server but follows Telegram's expected colon-separated
// shape so the bot library accepts it during construction.
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

// parseCall decodes one incoming HTTP request into a [Call]. The
// httptest handler calls this once per request inside the recorder
// mutex.
//
// Behavior:
//   - URL shape "/bot<token>/<method>" → Method := <method>.
//     Returns an empty Method when the URL has fewer than two
//     path segments (defensive; should never happen against the
//     real bot library).
//   - Content-Type "multipart/form-data" with a < 8 MiB body →
//     captures every form value (first value per key) and every
//     uploaded file (first file's filename per key).
//   - Other Content-Types (no body, JSON, application/x-www-form-
//     urlencoded) → returns a Call with empty Fields and Files.
//     The bot library uses multipart for everything that matters
//     so this rarely loses data in practice.
//   - 8 MiB ParseMultipartForm cap is the test budget — production
//     would never push that much through one form upload but the
//     limit means a misbehaving test can't OOM the harness.
func parseCall(req *http.Request) Call {
	call := Call{Fields: map[string]string{}, Files: map[string]string{}}

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

// MustDecodeInlineKeyboard extracts the reply_markup field from a
// [Call], unmarshals it as [models.InlineKeyboardMarkup], and
// fails the test on error.
//
// Use in keyboard-shape assertions: read the relevant call from
// the recorder, hand it to this helper, walk the resulting
// keyboard rows.
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

// MustDecodeReplyKeyboard is the [models.ReplyKeyboardMarkup]
// counterpart of [MustDecodeInlineKeyboard]. Same fail-fast
// semantics.
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

// defaultOKResponse is the canned JSON body returned by [NewBot]'s
// stub server when no test has overridden [Recorder.Response]. The
// shape mirrors a successful sendMessage reply with the minimum
// fields the bot library needs to deserialise without panic
// (message_id, date, chat).
var defaultOKResponse = []byte(`{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}}`)
