package handlers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

// musicGetCmd / musicGetWithArtworkCmd are the runner.Fake
// keys for the per-tick read commands.
const (
	musicGetCmd         = "nowplaying-cli get title album artist duration elapsedTime playbackRate contentItemIdentifier"
	musicGetWithArtwork = musicGetCmd + " artworkData"
	soundMusGetCmd      = "osascript -e set v to output volume of (get volume settings)\n" +
		"set m to output muted of (get volume settings)\n" +
		"return (v as text) & \",\" & (m as text)"
)

// musicAllOK stages every read command the music handler may
// invoke. Saves per-test boilerplate; tests that need a
// per-action mutation override one entry.
func musicAllOK(h *harness) {
	h.Fake.
		On(musicGetCmd, "Song\nAlbum\nArtist\n200\n50\n1\nid-1\n", nil).
		On(musicGetWithArtwork, "Song\nAlbum\nArtist\n200\n50\n1\nid-1\n\n", nil).
		On(soundMusGetCmd, "60,false", nil)
}

func TestMus_OpenWithCLI_SendsPhoto(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:open")); err != nil {
		t.Fatal(err)
	}
	// Open path: deletes the prior message + sendPhoto.
	if got := len(h.Recorder.ByMethod("deleteMessage")); got != 1 {
		t.Errorf("expected 1 deleteMessage; got %d", got)
	}
	if got := len(h.Recorder.ByMethod("sendPhoto")); got != 1 {
		t.Fatalf("expected 1 sendPhoto; got %d", got)
	}
	cap := h.Recorder.ByMethod("sendPhoto")[0].Fields["caption"]
	if !strings.Contains(cap, "Song") {
		t.Errorf("photo caption missing track title; got %q", cap)
	}
	if !h.Deps.MusicRefresh.IsActive(42) {
		t.Error("expected refresher session active after open")
	}
}

func TestMus_OpenWithoutCLI_RendersInstallReminder(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// Disable the binary-presence flag.
	h.Deps.Capability.Features = capability.Features{
		NetworkQuality: true, Shortcuts: true, WdutilInfo: true, NowPlaying: false,
	}

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:open")); err != nil {
		t.Fatal(err)
	}
	// No photo / no delete; just an editMessageText with the install reminder.
	if got := len(h.Recorder.ByMethod("sendPhoto")); got != 0 {
		t.Errorf("must not sendPhoto when CLI missing; got %d", got)
	}
	if got := len(h.Recorder.ByMethod("editMessageText")); got != 1 {
		t.Fatalf("expected 1 editMessageText; got %d", got)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "nowplaying-cli") {
		t.Errorf("install reminder missing CLI name; got %q", text)
	}
	if h.Deps.MusicRefresh.IsActive(42) {
		t.Error("must NOT start refresher when CLI missing")
	}
}

func TestMus_PlayCallsServiceAndAcks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)
	h.Fake.On("nowplaying-cli play", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:play")); err != nil {
		t.Fatal(err)
	}
	if got := len(h.Recorder.ByMethod("answerCallbackQuery")); got == 0 {
		t.Error("expected an answerCallbackQuery (Ack) on play")
	}
	// Verify the verb actually fired.
	found := false
	for _, c := range h.Fake.Calls() {
		if c.Name == "nowplaying-cli" && len(c.Args) > 0 && c.Args[0] == "play" {
			found = true
		}
	}
	if !found {
		t.Errorf("nowplaying-cli play was not invoked; calls=%+v", h.Fake.Calls())
	}
}

func TestMus_VerbWithoutCLI_Toasts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Deps.Capability.Features = capability.Features{NowPlaying: false}

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:pause")); err != nil {
		t.Fatal(err)
	}
	// Toast = answerCallbackQuery WITH a text field.
	toasts := 0
	for _, c := range h.Recorder.ByMethod("answerCallbackQuery") {
		if c.Fields["text"] != "" {
			toasts++
		}
	}
	if toasts == 0 {
		t.Error("expected a toast when CLI missing")
	}
}

func TestMus_SeekInstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:seek")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Error("expected a Seek flow installed for chat 42")
	}
	if got := len(h.Recorder.ByMethod("sendMessage")); got != 1 {
		t.Fatalf("expected the typed-input prompt to send; got %d sendMessage", got)
	}
}

func TestMus_VolNudgeAdjustsAndRefreshes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)
	// Sound.Adjust performs Get + Set; both need stubs.
	h.Fake.On("osascript -e set volume output volume 65", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:vol-up:5")); err != nil {
		t.Fatal(err)
	}
	// Verify the osascript was actually issued.
	found := false
	for _, c := range h.Fake.Calls() {
		if len(c.Args) == 2 && c.Args[1] == "set volume output volume 65" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected osascript to set volume to 65; calls=%+v", h.Fake.Calls())
	}
}

func TestMus_VolMuteCallsService(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)
	h.Fake.On("osascript -e set volume output muted true", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:vol-mute")); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range h.Fake.Calls() {
		if len(c.Args) == 2 && c.Args[1] == "set volume output muted true" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mute osascript; calls=%+v", h.Fake.Calls())
	}
}

func TestMus_UnknownActionToast(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:bogus")); err != nil {
		t.Fatal(err)
	}
	toasts := h.Recorder.ByMethod("answerCallbackQuery")
	hit := false
	for _, c := range toasts {
		if strings.Contains(c.Fields["text"], "Unknown music") {
			hit = true
		}
	}
	if !hit {
		t.Error("expected 'Unknown music action.' toast")
	}
}

func TestMus_NavigateAwayStopsRefresher(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	musicAllOK(h)

	// Open Music → starts a refresher session for chat 42.
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "mus:open")); err != nil {
		t.Fatal(err)
	}
	if !h.Deps.MusicRefresh.IsActive(42) {
		t.Fatal("precondition: open should have started the refresher")
	}

	// Now route a non-music callback (Sound) — refresher must Stop.
	h.Fake.On(soundMusGetCmd, "60,false", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id-2", "snd:open")); err != nil {
		t.Fatal(err)
	}
	if h.Deps.MusicRefresh.IsActive(42) {
		t.Error("non-music callback must Stop the refresher session for that chat")
	}
}
