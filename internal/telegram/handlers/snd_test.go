package handlers_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

func TestSnd_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(soundGetScript, "60,false", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:open")); err != nil {
		t.Fatal(err)
	}
	edits := h.Recorder.ByMethod("editMessageText")
	if len(edits) != 1 {
		t.Fatalf("expected 1 editMessageText, got %d", len(edits))
	}
	if !strings.Contains(edits[0].Fields["text"], "60%") {
		t.Errorf("text = %q", edits[0].Fields["text"])
	}
}

func TestSnd_Up(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On(soundGetScript, "60,false", nil).
		On("osascript -e set volume output volume 65", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:up:5")); err != nil {
		t.Fatal(err)
	}
	if len(h.Recorder.ByMethod("editMessageText")) != 1 {
		t.Fatal("expected editMessageText")
	}
}

func TestSnd_Down(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On(soundGetScript, "50,false", nil).
		On("osascript -e set volume output volume 49", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:down:1")); err != nil {
		t.Fatal(err)
	}
}

func TestSnd_Max(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("osascript -e set volume output volume 100", "", nil).
		On(soundGetScript, "100,false", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:max")); err != nil {
		t.Fatal(err)
	}
}

func TestSnd_Mute(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("osascript -e set volume output muted true", "", nil).
		On(soundGetScript, "50,true", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:mute")); err != nil {
		t.Fatal(err)
	}
}

func TestSnd_Unmute(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("osascript -e set volume output muted false", "", nil).
		On(soundGetScript, "50,false", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:unmute")); err != nil {
		t.Fatal(err)
	}
}

func TestSnd_Set_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:set")); err != nil {
		t.Fatal(err)
	}
	// A flow should be installed for chat 42.
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Error("expected a flow to be installed after snd:set")
	}
	// And a prompt message should have been sent.
	if len(h.Recorder.ByMethod("sendMessage")) != 1 {
		t.Fatal("expected prompt sendMessage")
	}
}

func TestSnd_OpenRunnerError(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(soundGetScript, "", errors.New("boom"))

	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:open"))
	// errEdit edits the message with a warning.
	last := h.Recorder.Last()
	if last.Method != "editMessageText" || !strings.Contains(last.Fields["text"], "unavailable") {
		t.Fatalf("expected unavailable edit, got %+v", last)
	}
}

func TestSnd_Unknown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "snd:bogus"))
	// Should toast with "Unknown sound action".
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Error("expected answerCallbackQuery toast")
	}
}
