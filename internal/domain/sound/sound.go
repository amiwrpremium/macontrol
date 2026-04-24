// Package sound exposes macOS audio output controls — volume,
// mute, and text-to-speech.
//
// Every operation is backed by `osascript` (volume + mute via
// AppleScript's `volume settings`) or the built-in `say`
// (text-to-speech) so the package works on every ASi-compatible
// macOS release with zero brew dependencies.
//
// Public surface:
//
//   - [State] — the read-side snapshot (level + mute).
//   - [Service] — the per-process control surface; one instance
//     on bot.Deps.Services.Sound.
//
// All read methods refresh from osascript on every call (no
// cache); all write methods follow up with a Get so callers
// always see the post-change state.
package sound

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// State is the read-side snapshot of the macOS output device.
// Returned by [Service.Get] and by every write method (which
// internally calls Get to refresh).
//
// Lifecycle:
//   - Constructed by Service.Get on every call. Never cached.
//
// Field roles:
//   - Level is the volume in 0..100. Always clamped before
//     return so callers can rely on the range.
//   - Muted is true when the output is currently muted.
type State struct {
	// Level is the output volume on a 0..100 scale.
	Level int

	// Muted reports whether the output is currently muted.
	Muted bool
}

// Service is the macOS sound control surface. One instance per
// process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Sound.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     Note that osascript-based volume changes from multiple
//     goroutines could race observably (read-modify-write in
//     [Service.Adjust] or [Service.ToggleMute] is not atomic),
//     but the Telegram dispatcher serialises one update at a
//     time per chat so this isn't a practical concern.
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

// Get reads the current volume + mute state via a single
// AppleScript that returns "<level>,<muted>".
//
// Behavior:
//   - Runs an osascript that emits "55,false" (or similar).
//   - Splits on ',' and parses the level via strconv.Atoi (then
//     clamped via [clamp] for safety) and the muted flag by
//     comparing the trimmed second half against "true".
//   - Returns "unexpected volume output: %q" when the result
//     doesn't split into exactly two parts.
//   - Returns "parse volume: %w" wrapping the strconv.Atoi
//     error when the level isn't an int.
func (s *Service) Get(ctx context.Context) (State, error) {
	out, err := s.r.Exec(ctx, "osascript", "-e",
		"set v to output volume of (get volume settings)\n"+
			"set m to output muted of (get volume settings)\n"+
			"return (v as text) & \",\" & (m as text)")
	if err != nil {
		return State{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 2 {
		return State{}, fmt.Errorf("unexpected volume output: %q", out)
	}
	level, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return State{}, fmt.Errorf("parse volume: %w", err)
	}
	return State{
		Level: clamp(level),
		Muted: strings.TrimSpace(parts[1]) == "true",
	}, nil
}

// Set writes an absolute volume in 0..100, clamping out-of-range
// input via [clamp]. Returns the post-change [State] via
// [Service.Get] so callers always observe the new value.
//
// Behavior:
//   - Clamps level into 0..100 before passing to osascript.
//   - Runs `osascript -e 'set volume output volume <level>'`.
//   - On osascript success, calls Get to refresh and returns
//     its result.
func (s *Service) Set(ctx context.Context, level int) (State, error) {
	level = clamp(level)
	_, err := s.r.Exec(ctx, "osascript", "-e",
		fmt.Sprintf("set volume output volume %d", level))
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx)
}

// Adjust shifts the current volume by delta (positive or
// negative), clamping the result into 0..100. Composes
// [Service.Get] with [Service.Set]; the read-modify-write is
// not atomic but the Telegram dispatcher serialises updates per
// chat so it doesn't race in practice.
//
// Returns the post-change [State].
func (s *Service) Adjust(ctx context.Context, delta int) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.Set(ctx, cur.Level+delta)
}

// Max sets volume to 100. Convenience over `Set(ctx, 100)`.
// Used by the Sound dashboard's MAX button.
func (s *Service) Max(ctx context.Context) (State, error) { return s.Set(ctx, 100) }

// Mute sets the output muted flag to true. Returns the
// post-change [State].
func (s *Service) Mute(ctx context.Context) (State, error) { return s.setMuted(ctx, true) }

// Unmute sets the output muted flag to false. Returns the
// post-change [State].
func (s *Service) Unmute(ctx context.Context) (State, error) { return s.setMuted(ctx, false) }

// ToggleMute flips the current mute flag. Composes
// [Service.Get] with [Service.setMuted]; not atomic with
// concurrent volume changes (see [Service]'s concurrency note).
//
// Returns the post-change [State].
func (s *Service) ToggleMute(ctx context.Context) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.setMuted(ctx, !cur.Muted)
}

// Say speaks text through the default macOS text-to-speech
// voice via the built-in `say` CLI.
//
// Behavior:
//   - Runs `say <text>` with text as a single positional arg
//     so multi-word phrases work without quoting concerns.
//   - Returns the runner error verbatim on `say` failure (rare;
//     `say` is reliable).
//   - Does NOT validate the text — empty strings are passed to
//     `say` which produces no audible output.
//
// Note: `say` blocks until speech completes; long inputs hold
// the call for the duration of the spoken text.
func (s *Service) Say(ctx context.Context, text string) error {
	_, err := s.r.Exec(ctx, "say", text)
	return err
}

// setMuted is the shared implementation behind [Service.Mute],
// [Service.Unmute], and [Service.ToggleMute]. Runs
// `osascript -e 'set volume output muted <bool>'` and refreshes
// via [Service.Get].
func (s *Service) setMuted(ctx context.Context, muted bool) (State, error) {
	_, err := s.r.Exec(ctx, "osascript", "-e",
		fmt.Sprintf("set volume output muted %t", muted))
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx)
}

// clamp constrains v into the 0..100 volume range used
// throughout the package. Values below 0 saturate to 0; above
// 100 saturate to 100; in-range values pass through unchanged.
func clamp(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}
