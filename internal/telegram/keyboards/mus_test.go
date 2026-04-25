package keyboards_test

import (
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

func TestMusicCaption_MissingCLI(t *testing.T) {
	t.Parallel()
	got := keyboards.MusicCaption(music.NowPlaying{}, sound.State{}, false)
	if !strings.Contains(got, "not installed") {
		t.Errorf("expected install reminder; got %q", got)
	}
	if !strings.Contains(got, "brew install nowplaying-cli") {
		t.Errorf("expected install command; got %q", got)
	}
}

func TestMusicCaption_NothingPlaying(t *testing.T) {
	t.Parallel()
	got := keyboards.MusicCaption(music.NowPlaying{}, sound.State{Level: 60}, true)
	if !strings.Contains(got, "Nothing playing") {
		t.Errorf("expected nothing-playing banner; got %q", got)
	}
	if !strings.Contains(got, "60%") {
		t.Errorf("volume footer missing in nothing-playing state; got %q", got)
	}
}

func TestMusicCaption_FullTrack(t *testing.T) {
	t.Parallel()
	np := music.NowPlaying{
		Title:        "Mr Brightside",
		Artist:       "The Killers",
		Album:        "Hot Fuss",
		Duration:     222 * time.Second,
		Elapsed:      63 * time.Second,
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{Level: 50}, true)
	for _, frag := range []string{
		"Mr Brightside", "The Killers", "Hot Fuss",
		"Passed: `1:03`", "Remaining: `2:39`", "▰", "▱", "🔊 `50%`",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("caption missing %q\nfull: %s", frag, got)
		}
	}
	if strings.Contains(got, "Paused") {
		t.Errorf("playing track must NOT show paused banner")
	}
}

func TestMusicCaption_Paused(t *testing.T) {
	t.Parallel()
	np := music.NowPlaying{
		Title:        "Track",
		Duration:     200 * time.Second,
		Elapsed:      100 * time.Second,
		PlaybackRate: 0.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	if !strings.Contains(got, "Paused") {
		t.Errorf("expected paused banner; got %q", got)
	}
}

func TestMusicCaption_HourLong(t *testing.T) {
	t.Parallel()
	np := music.NowPlaying{
		Title:        "Long Episode",
		Duration:     3 * time.Hour,
		Elapsed:      90 * time.Minute,
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	if !strings.Contains(got, "1:30:00") {
		t.Errorf("expected h:mm:ss for long elapsed; got %q", got)
	}
	if !strings.Contains(got, "1:30:00") {
		t.Errorf("expected h:mm:ss remaining; got %q", got)
	}
}

func TestMusicCaption_EscapesMarkdown(t *testing.T) {
	t.Parallel()
	np := music.NowPlaying{
		Title:        "track_with_underscores",
		Artist:       "tame_impala",
		Duration:     180 * time.Second,
		Elapsed:      0,
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	if strings.Contains(got, "track_with_underscores") {
		t.Errorf("underscores must be escaped to avoid breaking markdown italics; got %q", got)
	}
	if !strings.Contains(got, "track\\_with\\_underscores") {
		t.Errorf("expected escaped underscores in title; got %q", got)
	}
}

func TestMusicCaption_ZeroDurationAllEmptyBar(t *testing.T) {
	t.Parallel()
	// When duration is 0 but elapsed > 0 (live stream pulling in some elapsed),
	// the bar should still render — all empty.
	np := music.NowPlaying{
		Title:        "Stream",
		Elapsed:      30 * time.Second,
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	// duration 0 + elapsed > 0 still renders the bar (12 ▱).
	if !strings.Contains(got, strings.Repeat("▱", 12)) {
		t.Errorf("expected 12-segment empty bar for zero-duration stream; got %q", got)
	}
}

func TestMusicCaption_OverElapsedFullBar(t *testing.T) {
	t.Parallel()
	// elapsed > duration (player drift) — bar should clamp to full.
	np := music.NowPlaying{
		Title:        "T",
		Duration:     100 * time.Second,
		Elapsed:      150 * time.Second,
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	if !strings.Contains(got, strings.Repeat("▰", 12)) {
		t.Errorf("expected 12-segment full bar when elapsed > duration; got %q", got)
	}
	// Remaining should clamp to 0:00 rather than going negative.
	if !strings.Contains(got, "Remaining: `0:00`") {
		t.Errorf("expected Remaining: 0:00 for negative remainder; got %q", got)
	}
}

func TestMusicCaption_VolumeMutedFooter(t *testing.T) {
	t.Parallel()
	got := keyboards.MusicCaption(music.NowPlaying{}, sound.State{Level: 50, Muted: true}, true)
	if !strings.Contains(got, "MUTED") {
		t.Errorf("expected MUTED in footer; got %q", got)
	}
}

func TestMusicCaption_PartialMetadata(t *testing.T) {
	t.Parallel()
	// Title-only — no artist or album.
	np := music.NowPlaying{
		Title:        "Live Stream",
		PlaybackRate: 1.0,
	}
	got := keyboards.MusicCaption(np, sound.State{}, true)
	if !strings.Contains(got, "Live Stream") {
		t.Errorf("expected title; got %q", got)
	}
	// No "Passed:" / "Remaining:" since duration == 0.
	if strings.Contains(got, "Passed:") {
		t.Errorf("zero-duration track should NOT render progress; got %q", got)
	}
}

func TestMusicKeyboard_MissingCLI(t *testing.T) {
	t.Parallel()
	kb := keyboards.MusicKeyboard(false, false, false)
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected single Nav row when CLI missing; got %d rows", len(kb.InlineKeyboard))
	}
}

func TestMusicKeyboard_PlayingFlipsButton(t *testing.T) {
	t.Parallel()
	kb := keyboards.MusicKeyboard(true, false, true)
	playPauseRow := kb.InlineKeyboard[0]
	if len(playPauseRow) != 3 {
		t.Fatalf("expected 3 buttons in playback row; got %d", len(playPauseRow))
	}
	if playPauseRow[1].Text != "⏯ Pause" {
		t.Errorf("playing state should show Pause; got %q", playPauseRow[1].Text)
	}
	if !strings.HasSuffix(playPauseRow[1].CallbackData, ":pause") {
		t.Errorf("Pause callback should end in :pause; got %q", playPauseRow[1].CallbackData)
	}
}

func TestMusicKeyboard_PausedFlipsButton(t *testing.T) {
	t.Parallel()
	kb := keyboards.MusicKeyboard(false, false, true)
	playPauseRow := kb.InlineKeyboard[0]
	if playPauseRow[1].Text != "⏯ Play" {
		t.Errorf("paused state should show Play; got %q", playPauseRow[1].Text)
	}
}

func TestMusicKeyboard_MutedFlipsRow(t *testing.T) {
	t.Parallel()
	kb := keyboards.MusicKeyboard(true, true, true)
	volRow := kb.InlineKeyboard[1]
	if volRow[2].Text != "🔈 Unmute" {
		t.Errorf("muted state should show Unmute in middle slot; got %q", volRow[2].Text)
	}
}

func TestMusicKeyboard_AllVolumeCallbacksAreNamespacedToMus(t *testing.T) {
	t.Parallel()
	// Embedded sound controls MUST stay on the mus namespace so
	// navigating away from Music actually fires a non-music
	// callback that stops the refresher.
	kb := keyboards.MusicKeyboard(true, false, true)
	volRow := kb.InlineKeyboard[1]
	for _, btn := range volRow {
		if !strings.HasPrefix(btn.CallbackData, "mus:") {
			t.Errorf("volume button callback must start with mus:; got %q", btn.CallbackData)
		}
	}
}

func TestMusicKeyboard_HasSeekAndRefresh(t *testing.T) {
	t.Parallel()
	kb := keyboards.MusicKeyboard(true, false, true)
	if len(kb.InlineKeyboard) != 4 {
		t.Fatalf("expected 4 rows; got %d", len(kb.InlineKeyboard))
	}
	row3 := kb.InlineKeyboard[2]
	if row3[0].Text != "⏩ Seek…" {
		t.Errorf("expected Seek… in row 3; got %q", row3[0].Text)
	}
	if row3[1].Text != "🔄 Refresh" {
		t.Errorf("expected Refresh in row 3; got %q", row3[1].Text)
	}
}
