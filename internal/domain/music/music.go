// Package music exposes player-agnostic playback control + now-
// playing metadata via the third-party `nowplaying-cli` binary.
//
// The binary wraps Apple's private `MediaRemote.framework`, so
// the package works against every macOS media player that
// publishes Now Playing info (Music.app, Spotify, Podcasts,
// browsers via OS-level controls, …) without per-app glue.
//
// Public surface:
//
//   - [NowPlaying] — the read-side snapshot (metadata, position,
//     artwork bytes).
//   - [Service] — the per-process control surface; one instance
//     on bot.Deps.Services.Music.
//
// All read methods refresh from `nowplaying-cli` on every call
// (no cache). All write methods are fire-and-forget; callers
// re-Get to observe the post-change state.
package music

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// NowPlaying is the read-side snapshot of the macOS Now Playing
// session. Returned by [Service.Get] and (with [NowPlaying.Artwork]
// populated) by [Service.GetWithArtwork].
//
// Lifecycle:
//   - Constructed by Service.Get on every call. Never cached
//     across calls; the in-memory copy held by the refresher
//     goroutine is its own short-lived snapshot.
//
// Field roles:
//   - Title / Album / Artist are best-effort metadata; any may
//     be empty when the active player doesn't publish that
//     field (e.g. some podcast apps skip Album).
//   - Duration / Elapsed are the player-reported position at
//     fetch time, NOT a wall-clock extrapolation. The refresher
//     re-fetches every 5 s rather than incrementing locally.
//   - PlaybackRate is 1.0 for normal play, 0.0 when paused,
//     >1.0 when scrubbing forward, <0 when scrubbing back.
//     [NowPlaying.IsPlaying] is the convenience predicate.
//   - TrackID is the verbatim contentItemIdentifier from
//     MediaRemote — used by the refresher to detect track
//     changes and swap the photo via editMessageMedia.
//   - Artwork is the raw PNG bytes from artworkData. Only
//     populated by [Service.GetWithArtwork]; nil from plain
//     [Service.Get] to keep the per-tick payload small.
type NowPlaying struct {
	// Title is the track / episode / chapter title, or empty
	// when the player doesn't publish one.
	Title string

	// Album is the album / show / collection name, or empty
	// when the player doesn't publish one (common on podcasts).
	Album string

	// Artist is the artist / host / performer name, or empty
	// when the player doesn't publish one.
	Artist string

	// Duration is the total length of the current item.
	// Zero when the player doesn't publish a duration (e.g.
	// live streams).
	Duration time.Duration

	// Elapsed is the player-reported playback position at
	// fetch time. Zero is "at start" or "unknown" — the
	// caller can't distinguish the two.
	Elapsed time.Duration

	// PlaybackRate is 1.0 for normal play, 0.0 when paused,
	// other values when scrubbing. Use [NowPlaying.IsPlaying]
	// for the boolean.
	PlaybackRate float64

	// TrackID is the MediaRemote contentItemIdentifier — the
	// stable per-track id the refresher uses to detect track
	// changes. Empty when the player doesn't publish one (rare).
	TrackID string

	// Artwork is the raw PNG bytes for the track artwork.
	// Nil when fetched via [Service.Get]; populated when
	// fetched via [Service.GetWithArtwork]. Also nil when the
	// active player doesn't expose artwork (some podcast apps).
	Artwork []byte
}

// IsPlaying reports whether the player is actively playing
// (PlaybackRate > 0). False when paused (rate == 0), scrubbing
// backward (rate < 0), or when no track is loaded.
func (n NowPlaying) IsPlaying() bool { return n.PlaybackRate > 0 }

// Service is the macOS Now Playing control surface. One
// instance per process, shared across chats.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Music.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     Multiple chats refreshing simultaneously each spawn
//     their own subprocess via the runner; the runner is
//     itself concurrent-safe.
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// metadataFields is the field list passed to `nowplaying-cli get`
// for the lightweight (no-artwork) snapshot. The order is the
// contract: line N of stdout is metadataFields[N].
var metadataFields = []string{
	"title",
	"album",
	"artist",
	"duration",
	"elapsedTime",
	"playbackRate",
	"contentItemIdentifier",
}

// Get reads metadata + position via
// `nowplaying-cli get title album artist duration elapsedTime playbackRate contentItemIdentifier`.
//
// Behavior:
//   - Subprocess returns one value per requested field on its
//     own line. Empty lines map cleanly to Go zero-values
//     (empty strings for text, zero for numbers).
//   - Numeric fields (duration, elapsedTime, playbackRate)
//     arrive as plain decimal strings; parsed via
//     [parseFloat] and converted to time.Duration via
//     [parseSecs] for the time fields.
//   - Returns the runner error verbatim on subprocess failure
//     (most commonly "executable file not found in $PATH" when
//     nowplaying-cli isn't installed — handlers surface the
//     install hint).
//
// Artwork is intentionally NOT requested here; use
// [Service.GetWithArtwork] when you need it (the per-tick
// refresher uses Get to keep the payload small).
func (s *Service) Get(ctx context.Context) (NowPlaying, error) {
	out, err := s.r.Exec(ctx, "nowplaying-cli", append([]string{"get"}, metadataFields...)...)
	if err != nil {
		return NowPlaying{}, err
	}
	return parseSnapshot(string(out)), nil
}

// GetWithArtwork is [Service.Get] plus the artworkData base64
// blob decoded into [NowPlaying.Artwork]. Used by the refresher
// on first render and on track change so the per-tick path can
// stay on the lighter [Service.Get].
//
// Behavior:
//   - Asks `nowplaying-cli get` for the metadata fields plus
//     `artworkData` as the trailing field.
//   - Decodes the base64 line into [NowPlaying.Artwork]. An
//     empty line (player publishes no artwork) leaves Artwork
//     nil with no error.
//   - Returns "decode artwork: %w" wrapping the
//     [base64.StdEncoding] error when the trailing line is
//     non-empty but malformed (rare; nowplaying-cli's output
//     is stable).
func (s *Service) GetWithArtwork(ctx context.Context) (NowPlaying, error) {
	args := append([]string{"get"}, metadataFields...)
	args = append(args, "artworkData")
	out, err := s.r.Exec(ctx, "nowplaying-cli", args...)
	if err != nil {
		return NowPlaying{}, err
	}
	np := parseSnapshot(string(out))
	lines := strings.Split(string(out), "\n")
	if len(lines) > len(metadataFields) {
		raw := strings.TrimSpace(lines[len(metadataFields)])
		if raw != "" {
			art, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				return np, fmt.Errorf("decode artwork: %w", err)
			}
			np.Artwork = art
		}
	}
	return np, nil
}

// Play runs `nowplaying-cli play`. Resumes the current track
// from its paused position, or starts the queued track when
// nothing is playing.
//
// Behavior:
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new state.
func (s *Service) Play(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "nowplaying-cli", "play")
	return err
}

// Pause runs `nowplaying-cli pause`. Holds the current playback
// position; resume via [Service.Play] or [Service.TogglePlayPause].
//
// Behavior:
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new state.
func (s *Service) Pause(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "nowplaying-cli", "pause")
	return err
}

// TogglePlayPause runs `nowplaying-cli togglePlayPause`. The
// player's current state determines whether this resumes or
// pauses; the bot doesn't track state itself.
//
// Behavior:
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new state.
func (s *Service) TogglePlayPause(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "nowplaying-cli", "togglePlayPause")
	return err
}

// Next runs `nowplaying-cli next`. Advances to the next track
// in the player's queue.
//
// Behavior:
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new track.
//     The refresher detects the [NowPlaying.TrackID] change and
//     swaps the dashboard photo via editMessageMedia.
func (s *Service) Next(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "nowplaying-cli", "next")
	return err
}

// Previous runs `nowplaying-cli previous`. Per macOS / player
// convention this rewinds to the start of the current track if
// elapsed > a few seconds, otherwise jumps to the previous track.
//
// Behavior:
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new state.
func (s *Service) Previous(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "nowplaying-cli", "previous")
	return err
}

// Seek runs `nowplaying-cli seek <secs>`. Jumps the current
// track to the absolute position secs.
//
// Behavior:
//   - secs is the absolute position from the start of the track,
//     NOT a relative offset. Negative values are clamped to 0.
//   - Fire-and-forget; returns the runner error verbatim.
//   - Caller is expected to re-Get to observe the new position.
//     Some players ignore seek requests for live streams or
//     non-seekable content; nowplaying-cli surfaces no error in
//     that case.
func (s *Service) Seek(ctx context.Context, secs int) error {
	if secs < 0 {
		secs = 0
	}
	_, err := s.r.Exec(ctx, "nowplaying-cli", "seek", strconv.Itoa(secs))
	return err
}

// parseSnapshot decodes a positional `nowplaying-cli get`
// response into a [NowPlaying]. Splits stdout on '\n' and
// indexes by [metadataFields] position; missing or empty lines
// leave the corresponding field at its zero value. Never
// returns an error — the only failure modes are subprocess
// failures (handled by the caller before this is reached) and
// the artwork decode (handled separately in
// [Service.GetWithArtwork]).
func parseSnapshot(out string) NowPlaying {
	lines := strings.Split(out, "\n")
	return NowPlaying{
		Title:        line(lines, 0),
		Album:        line(lines, 1),
		Artist:       line(lines, 2),
		Duration:     parseSecs(line(lines, 3)),
		Elapsed:      parseSecs(line(lines, 4)),
		PlaybackRate: parseFloat(line(lines, 5)),
		TrackID:      line(lines, 6),
	}
}

// line returns the trimmed lines[i], or "" when i is out of
// bounds. Used by [parseSnapshot] to tolerate truncated
// `nowplaying-cli` output without panicking.
func line(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[i])
}

// parseSecs parses a fractional-second decimal string into a
// time.Duration. Empty input or parse failure both return 0;
// the parser is intentionally lenient so a single malformed
// field can't blank out the whole snapshot.
func parseSecs(s string) time.Duration {
	f := parseFloat(s)
	return time.Duration(f * float64(time.Second))
}

// parseFloat is strconv.ParseFloat with errors swallowed and
// empty input mapped to 0. Used for the lenient numeric
// parsing across [parseSnapshot] and [parseSecs].
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
