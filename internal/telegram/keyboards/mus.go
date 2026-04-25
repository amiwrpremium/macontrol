package keyboards

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// progressBarSegments is the unicode bar's character width used
// across every Music caption. 12 segments was picked because it
// fits comfortably under Telegram's caption-line wrapping on
// every common phone width (the bar plus its surrounding
// spaces still leaves room for both flanking time labels in
// edge cases).
const progressBarSegments = 12

// MusicCaption builds the photo-message caption for the 🎵
// Music dashboard.
//
// Behavior:
//
// Header rendering (first match wins):
//  1. !hasCLI → install reminder; the body block is skipped.
//  2. np.Title == "" → "🎵 *Music* — _Nothing playing_"; body
//     skipped.
//  3. Otherwise → "🎵 *<title>*\n_<artist>_ · `<album>`" with
//     each subordinate field elided when empty.
//
// Body rendering (only when there's an active track):
//   - Line 1: "Passed: <m:ss>" via [formatHMS].
//   - Line 2: 12-segment unicode bar via [progressBar].
//   - Line 3: "Remaining: <m:ss>".
//   - Volume footer: "🔊 <pct>% · <unmuted|MUTED>".
//
// Tracks ≥ 1h auto-extend to h:mm:ss via [formatHMS].
//
// The caption is the user-visible body of the photo message;
// it sits underneath the artwork and is what the refresher
// re-renders via editMessageCaption every 5 s.
func MusicCaption(np music.NowPlaying, vol sound.State, hasCLI bool) string {
	if !hasCLI {
		return "🎵 *Music* — _`nowplaying-cli` not installed_\n\nInstall via:\n`brew install nowplaying-cli`"
	}
	if np.Title == "" {
		return "🎵 *Music* — _Nothing playing_\n\n" + volumeFooter(vol)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🎵 *%s*", escapeMD(np.Title))
	if line := metadataSubline(np); line != "" {
		b.WriteString("\n")
		b.WriteString(line)
	}
	if np.Duration > 0 || np.Elapsed > 0 {
		b.WriteString("\n\n")
		b.WriteString(progressBlock(np))
	}
	if !np.IsPlaying() {
		b.WriteString("\n\n_⏸ Paused_")
	}
	b.WriteString("\n\n")
	b.WriteString(volumeFooter(vol))
	return b.String()
}

// metadataSubline composes the "_Artist_ · `Album`" line that
// sits under the title. Returns "" when neither field is set
// so the caller can elide the leading newline.
func metadataSubline(np music.NowPlaying) string {
	if np.Artist == "" && np.Album == "" {
		return ""
	}
	var b strings.Builder
	if np.Artist != "" {
		fmt.Fprintf(&b, "_%s_", escapeMD(np.Artist))
	}
	if np.Artist != "" && np.Album != "" {
		b.WriteString(" · ")
	}
	if np.Album != "" {
		fmt.Fprintf(&b, "`%s`", escapeMD(np.Album))
	}
	return b.String()
}

// progressBlock composes the three-line position block:
// "Passed: m:ss" / progressBar / "Remaining: m:ss". Caller is
// responsible for the leading separator since some callers
// want a double newline and others want the block inline.
func progressBlock(np music.NowPlaying) string {
	remaining := np.Duration - np.Elapsed
	if remaining < 0 {
		remaining = 0
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Passed: `%s`\n", formatHMS(np.Elapsed))
	b.WriteString(progressBar(np.Elapsed, np.Duration))
	b.WriteString("\n")
	fmt.Fprintf(&b, "Remaining: `%s`", formatHMS(remaining))
	return b.String()
}

// volumeFooter renders the embedded sound state line that
// appears at the bottom of every active Music caption.
func volumeFooter(vol sound.State) string {
	muted := "unmuted"
	if vol.Muted {
		muted = "MUTED"
	}
	return fmt.Sprintf("🔊 `%d%%` · %s", vol.Level, muted)
}

// MusicKeyboard returns the inline keyboard that pairs with
// [MusicCaption].
//
// Keyboard rendering (4 rows, hasCLI=true):
//  1. ⏮ Prev · ⏯ Play|Pause · ⏭ Next. The middle button text
//     and callback (`mus:play` vs `mus:pause`) flip with
//     isPlaying so a single button handles both verbs.
//  2. Volume nudge row mirrored from [Sound]: −5 / −1 /
//     [Mute|Unmute] / +1 / +5. Embedded so the user can
//     silence playback without bouncing back to Sound. All
//     callbacks use the NSMusic namespace (`mus:vol-*`) so
//     navigating away from Music stops the refresher.
//  3. ⏩ Seek… · 🔄 Refresh.
//  4. Standard Back/Home nav row via [NavWithBack].
//
// When hasCLI is false the playback row + volume row collapse
// into a single Nav-only keyboard so the user has nothing to
// tap until they install nowplaying-cli.
func MusicKeyboard(isPlaying, muted, hasCLI bool) *models.InlineKeyboardMarkup {
	if !hasCLI {
		return &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				NavWithBack(callbacks.NSNav, "home"),
			},
		}
	}

	playPause := models.InlineKeyboardButton{
		Text: "⏯ Pause", CallbackData: callbacks.Encode(callbacks.NSMusic, "pause"),
	}
	if !isPlaying {
		playPause = models.InlineKeyboardButton{
			Text: "⏯ Play", CallbackData: callbacks.Encode(callbacks.NSMusic, "play"),
		}
	}
	muteBtn := models.InlineKeyboardButton{
		Text: "🔇 Mute", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-mute"),
	}
	if muted {
		muteBtn = models.InlineKeyboardButton{
			Text: "🔈 Unmute", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-unmute"),
		}
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "⏮ Prev", CallbackData: callbacks.Encode(callbacks.NSMusic, "prev")},
				playPause,
				{Text: "⏭ Next", CallbackData: callbacks.Encode(callbacks.NSMusic, "next")},
			},
			{
				{Text: "−5", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-down", "5")},
				{Text: "−1", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-down", "1")},
				muteBtn,
				{Text: "+1", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-up", "1")},
				{Text: "+5", CallbackData: callbacks.Encode(callbacks.NSMusic, "vol-up", "5")},
			},
			{
				{Text: "⏩ Seek…", CallbackData: callbacks.Encode(callbacks.NSMusic, "seek")},
				{Text: "🔄 Refresh", CallbackData: callbacks.Encode(callbacks.NSMusic, "refresh")},
			},
			NavWithBack(callbacks.NSNav, "home"),
		},
	}
}

// progressBar renders the elapsed/duration ratio as a
// fixed-width unicode bar of [progressBarSegments] characters
// using ▰ for filled and ▱ for empty.
//
// Behavior:
//   - duration <= 0 → all-empty bar (nothing to compute against).
//   - elapsed >= duration → all-filled bar.
//   - otherwise → floor(elapsed/duration * segments) filled
//     characters followed by the empty remainder.
func progressBar(elapsed, duration time.Duration) string {
	if duration <= 0 {
		return strings.Repeat("▱", progressBarSegments)
	}
	filled := int(float64(elapsed) / float64(duration) * float64(progressBarSegments))
	if filled < 0 {
		filled = 0
	}
	if filled > progressBarSegments {
		filled = progressBarSegments
	}
	return strings.Repeat("▰", filled) + strings.Repeat("▱", progressBarSegments-filled)
}

// formatHMS renders d as "m:ss" for durations < 1 h or
// "h:mm:ss" for longer ones. Used on every visible time label
// in the Music caption.
func formatHMS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// escapeMD escapes the small set of Telegram-Markdown metachars
// that can appear in track metadata (most commonly underscores
// in artist names like `tame_impala`). Backticks and asterisks
// are not present in real-world track titles often enough to
// justify escaping; if they do appear, they fall through into
// the rendered caption looking like markdown but the message
// still sends without rejection.
func escapeMD(s string) string {
	r := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"`", "\\`",
	)
	return r.Replace(s)
}
