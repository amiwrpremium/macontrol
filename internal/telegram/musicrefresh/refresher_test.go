package musicrefresh_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/musicrefresh"
	"github.com/amiwrpremium/macontrol/internal/telegram/telegramtest"
)

// canned wires a music + sound service against a single shared
// runner.Fake so a Manager-level test can stage stdout per
// command and observe call ordering across both services.
func canned(t *testing.T) (*runner.Fake, *music.Service, *sound.Service) {
	t.Helper()
	f := runner.NewFake()
	return f, music.New(f), sound.New(f)
}

// musicRule + soundRule register the canned stdout for the
// per-tick read commands. Helpers keep the per-test wiring
// short; production-shaped strings live in mus_test.go and
// sound_test.go fixtures.
const (
	musicGetCmd  = "nowplaying-cli get title album artist duration elapsedTime playbackRate contentItemIdentifier"
	musicGetWArt = musicGetCmd + " artworkData"
	soundGetCmd  = "osascript -e set v to output volume of (get volume settings)\n" +
		"set m to output muted of (get volume settings)\n" +
		"return (v as text) & \",\" & (m as text)"
)

func TestPlaceholder_IsValidPNG(t *testing.T) {
	t.Parallel()
	if len(musicrefresh.Placeholder) < 8 {
		t.Fatalf("placeholder too short: %d bytes", len(musicrefresh.Placeholder))
	}
	pngHead := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	for i, b := range pngHead {
		if musicrefresh.Placeholder[i] != b {
			t.Fatalf("placeholder is not a PNG (byte %d = %x, want %x)", i, musicrefresh.Placeholder[i], b)
		}
	}
}

func TestNewManager_DefaultsAreSet(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	// IsActive on an unknown chat must be false; this also
	// exercises the public read path.
	if m.IsActive(42) {
		t.Fatal("fresh manager should have no active chats")
	}
}

func TestStart_TracksChat(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	// Use a long tick so the goroutine doesn't actually edit
	// anything during the test; we only verify Start records
	// the chat as active.
	m.SetTick(1 * time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "init"})
	defer m.Stop(42)

	if !m.IsActive(42) {
		t.Fatal("expected chat 42 active after Start")
	}
}

func TestStart_CancelsExistingSession(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetTick(1 * time.Hour)

	ctx := context.Background()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "first"})
	// Second Start should silently replace the first.
	m.Start(ctx, 42, 8, music.NowPlaying{TrackID: "second"})
	defer m.Stop(42)

	if !m.IsActive(42) {
		t.Fatal("expected chat 42 active after replacement Start")
	}
}

func TestStop_ReturnsTrueOnce(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetTick(1 * time.Hour)

	m.Start(context.Background(), 42, 7, music.NowPlaying{})
	if !m.Stop(42) {
		t.Fatal("first Stop should report a session was cancelled")
	}
	if m.Stop(42) {
		t.Fatal("second Stop on the same chat should report nothing to cancel")
	}
	if m.IsActive(42) {
		t.Fatal("Stop must clear the active map entry")
	}
}

func TestStop_NoOpOnMissingChat(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	if m.Stop(999) {
		t.Fatal("Stop on unknown chat must return false")
	}
}

func TestTouch_NoSessionIsNoOp(t *testing.T) {
	t.Parallel()
	_, ms, ss := canned(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	// Just shouldn't panic; nothing to assert.
	m.Touch(123, music.NowPlaying{}, sound.State{})
}

func TestTickEditsCaption(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	// Steady state: a short paused track. No track change so the
	// tick path goes through editMessageCaption.
	f.On(musicGetCmd, "Song\nAlbum\nArtist\n200\n100\n0\nid-1\n", nil).
		On(soundGetCmd, "60,false", nil)

	bot, rec := telegramtest.NewBot(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetBot(bot)
	m.SetTick(20 * time.Millisecond)
	m.SetMax(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "id-1"})
	defer m.Stop(42)

	waitFor(t, 500*time.Millisecond, func() bool {
		return len(rec.ByMethod("editMessageCaption")) > 0
	})

	calls := rec.ByMethod("editMessageCaption")
	if len(calls) == 0 {
		t.Fatal("expected at least one editMessageCaption call")
	}
	cap := calls[0].Fields["caption"]
	if !strings.Contains(cap, "Song") || !strings.Contains(cap, "1:40") {
		t.Errorf("caption missing expected fragments: %q", cap)
	}
	if rec.ByMethod("editMessageMedia") != nil && len(rec.ByMethod("editMessageMedia")) > 0 {
		t.Errorf("expected NO editMessageMedia calls without a track change; got %d", len(rec.ByMethod("editMessageMedia")))
	}
}

func TestTickSwapsMediaOnTrackChange(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	// Initial trackID is id-old; the next tick reports id-new
	// which forces a GetWithArtwork → editMessageMedia path.
	f.On(musicGetCmd, "New Song\nA\nR\n100\n5\n1\nid-new\n", nil).
		On(musicGetWArt, "New Song\nA\nR\n100\n5\n1\nid-new\n\n", nil).
		On(soundGetCmd, "30,false", nil)

	bot, rec := telegramtest.NewBot(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetBot(bot)
	m.SetTick(20 * time.Millisecond)
	m.SetMax(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "id-old"})
	defer m.Stop(42)

	waitFor(t, 500*time.Millisecond, func() bool {
		return len(rec.ByMethod("editMessageMedia")) > 0
	})

	if got := len(rec.ByMethod("editMessageMedia")); got == 0 {
		t.Fatal("expected editMessageMedia on track change; got none")
	}
}

func TestTickWithoutBotSkipsButKeepsRunning(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	f.On(musicGetCmd, "T\nA\nR\n100\n50\n1\nid\n", nil).
		On(soundGetCmd, "60,false", nil)

	m := musicrefresh.NewManager(ms, ss, nil) // no SetBot
	m.SetTick(20 * time.Millisecond)
	m.SetMax(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "id"})

	// Wait the deadline; no panic, no edit calls — session stayed alive.
	<-ctx.Done()
	// Give the goroutine a moment to evict.
	waitFor(t, 200*time.Millisecond, func() bool {
		return !m.IsActive(42)
	})
}

func TestMaxDurationSelfStops(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	f.On(musicGetCmd, "T\nA\nR\n100\n50\n1\nid\n", nil).
		On(soundGetCmd, "60,false", nil)

	bot, _ := telegramtest.NewBot(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetBot(bot)
	m.SetTick(10 * time.Millisecond)
	m.SetMax(40 * time.Millisecond)

	m.Start(context.Background(), 42, 7, music.NowPlaying{TrackID: "id"})

	waitFor(t, 500*time.Millisecond, func() bool {
		return !m.IsActive(42)
	})

	if m.IsActive(42) {
		t.Fatal("expected session to self-evict after max duration")
	}
}

func TestParentContextCancelsSession(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	f.On(musicGetCmd, "T\nA\nR\n100\n50\n1\nid\n", nil).
		On(soundGetCmd, "60,false", nil)

	bot, _ := telegramtest.NewBot(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetBot(bot)
	m.SetTick(50 * time.Millisecond)
	m.SetMax(1 * time.Hour)

	parent, cancel := context.WithCancel(context.Background())
	m.Start(parent, 42, 7, music.NowPlaying{TrackID: "id"})

	cancel() // simulate daemon shutdown.
	waitFor(t, 500*time.Millisecond, func() bool {
		return !m.IsActive(42)
	})
	if m.IsActive(42) {
		t.Fatal("parent context cancel must evict the session")
	}
}

func TestTouchPushesQueuedState(t *testing.T) {
	t.Parallel()
	f, ms, ss := canned(t)
	// If the touched state is honored the tick won't call music.Get
	// — but if the goroutine fell through to Get, the rule below
	// would still satisfy it. We assert the caption contents
	// instead, which only the Touch'd snapshot can produce.
	f.On(musicGetCmd, "OldFromGet\nA\nR\n100\n0\n1\nid\n", nil).
		On(soundGetCmd, "60,false", nil)

	bot, rec := telegramtest.NewBot(t)
	m := musicrefresh.NewManager(ms, ss, nil)
	m.SetBot(bot)
	m.SetTick(20 * time.Millisecond)
	m.SetMax(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx, 42, 7, music.NowPlaying{TrackID: "id"})
	defer m.Stop(42)

	m.Touch(42, music.NowPlaying{Title: "TouchedTitle", TrackID: "id", PlaybackRate: 1.0}, sound.State{Level: 88})

	waitFor(t, 500*time.Millisecond, func() bool {
		for _, c := range rec.ByMethod("editMessageCaption") {
			if strings.Contains(c.Fields["caption"], "TouchedTitle") {
				return true
			}
		}
		return false
	})
}

// waitFor polls cond every 10 ms up to timeout. Fails the test
// when cond never returns true. Used in lieu of arbitrary
// time.Sleep — reduces flake.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}
