package bot

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// stubRouter records invocations for dispatch assertions.
type stubRouter struct {
	mu      sync.Mutex
	called  bool
	wantErr error
}

func (s *stubRouter) Handle(_ context.Context, _ *Deps, _ *models.Update) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = true
	return s.wantErr
}

// stubFlowMgr implements FlowManager for dispatch tests.
type stubFlowMgr struct {
	flow flows.Flow
	has  bool
}

func (s *stubFlowMgr) Active(_ int64) (flows.Flow, bool) { return s.flow, s.has }
func (s *stubFlowMgr) Cancel(_ int64) bool               { s.has = false; return true }
func (s *stubFlowMgr) Install(_ int64, f flows.Flow)     { s.flow = f; s.has = true }
func (s *stubFlowMgr) Finish(_ int64)                    { s.has = false }

// fakeFlow consumes a single message.
type fakeFlow struct{ handled bool }

func (*fakeFlow) Name() string                         { return "test" }
func (*fakeFlow) Start(context.Context) flows.Response { return flows.Response{Text: "ask"} }
func (f *fakeFlow) Handle(context.Context, string) flows.Response {
	f.handled = true
	return flows.Response{Text: "ok", Done: true}
}

func newTestDeps() *Deps {
	return &Deps{
		Logger:    slog.New(slog.DiscardHandler),
		Whitelist: NewWhitelist([]int64{42}),
		Commands:  &stubRouter{},
		Calls:     &stubRouter{},
		Flows:     &stubFlowMgr{},
	}
}

// newRealBot constructs a *tgbot.Bot pointed at a dummy server URL. We don't
// actually need it to respond — dispatch doesn't itself make outbound calls
// unless a flow produces a reply.
func newRealBot(t *testing.T) *tgbot.Bot {
	t.Helper()
	b, err := tgbot.New("test-token:XYZ",
		tgbot.WithServerURL("http://127.0.0.1:1"), // unreachable — OK for routing-only tests
		tgbot.WithSkipGetMe(),
	)
	if err != nil {
		t.Fatalf("tgbot.New: %v", err)
	}
	return b
}

func TestDispatch_RejectsNonWhitelisted(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	u := &models.Update{
		Message: &models.Message{
			Text: "/menu",
			From: &models.User{ID: 999},
		},
	}
	d.dispatch(context.Background(), newRealBot(t), u)
	if d.Commands.(*stubRouter).called {
		t.Error("Commands router should not have been called")
	}
}

func TestDispatch_RoutesCallback(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	u := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			Data: "snd:refresh",
			From: models.User{ID: 42},
		},
	}
	d.dispatch(context.Background(), newRealBot(t), u)
	if !d.Calls.(*stubRouter).called {
		t.Error("Calls router should have been invoked")
	}
	if d.Commands.(*stubRouter).called {
		t.Error("Commands router should not have been invoked for callback")
	}
}

func TestDispatch_RoutesSlashCommand(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	u := &models.Update{
		Message: &models.Message{
			Text: "/status",
			From: &models.User{ID: 42},
		},
	}
	d.dispatch(context.Background(), newRealBot(t), u)
	if !d.Commands.(*stubRouter).called {
		t.Error("Commands router should have been invoked")
	}
}

func TestDispatch_PlainTextNoFlow(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	u := &models.Update{
		Message: &models.Message{
			Text: "hello",
			From: &models.User{ID: 42},
			Chat: models.Chat{ID: 42},
		},
	}
	d.dispatch(context.Background(), newRealBot(t), u)
	if d.Commands.(*stubRouter).called || d.Calls.(*stubRouter).called {
		t.Error("routers should not fire for plain text with no active flow")
	}
}

func TestDispatch_PlainTextConsumedByFlow(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	ff := &fakeFlow{}
	d.Flows = &stubFlowMgr{flow: ff, has: true}

	u := &models.Update{
		Message: &models.Message{
			Text: "body text",
			From: &models.User{ID: 42},
			Chat: models.Chat{ID: 42},
		},
	}
	// dispatch tries to send a reply via d.Bot.SendMessage. Our unreachable
	// URL will cause the send to fail and log a warning — but dispatch
	// itself doesn't surface that error. We only assert that the flow was
	// invoked.
	d.dispatch(context.Background(), newRealBot(t), u)
	if !ff.handled {
		t.Error("flow Handle should have been called")
	}
}

func TestDispatch_RouterError_IsLogged_NotPropagated(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.Commands = &stubRouter{wantErr: errors.New("boom")}
	u := &models.Update{
		Message: &models.Message{
			Text: "/lock",
			From: &models.User{ID: 42},
		},
	}
	// Must not panic; no assertion on error — dispatch swallows after logging.
	d.dispatch(context.Background(), newRealBot(t), u)
}

func TestDispatch_RecoversFromPanic(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.Commands = panicRouter{}
	u := &models.Update{
		Message: &models.Message{
			Text: "/lock",
			From: &models.User{ID: 42},
		},
	}
	// Should not panic.
	d.dispatch(context.Background(), newRealBot(t), u)
}

type panicRouter struct{}

func (panicRouter) Handle(_ context.Context, _ *Deps, _ *models.Update) error {
	panic("boom")
}
