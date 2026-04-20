// Package sound exposes macOS audio output controls — volume, mute, and
// text-to-speech. Everything is backed by osascript calls so it works on
// every ASi-compatible macOS release without brew dependencies.
package sound

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// State is a snapshot of the output device.
type State struct {
	Level int  // 0..100
	Muted bool
}

// Service controls macOS sound.
type Service struct{ r runner.Runner }

// New returns a Service backed by r.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Get reads current volume + mute state.
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

// Set writes an absolute volume (0..100).
func (s *Service) Set(ctx context.Context, level int) (State, error) {
	level = clamp(level)
	_, err := s.r.Exec(ctx, "osascript", "-e",
		fmt.Sprintf("set volume output volume %d", level))
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx)
}

// Adjust changes volume by delta (can be negative), clamped to [0,100].
func (s *Service) Adjust(ctx context.Context, delta int) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.Set(ctx, cur.Level+delta)
}

// Max sets volume to 100.
func (s *Service) Max(ctx context.Context) (State, error) { return s.Set(ctx, 100) }

// Mute sets output muted = true.
func (s *Service) Mute(ctx context.Context) (State, error) { return s.setMuted(ctx, true) }

// Unmute sets output muted = false.
func (s *Service) Unmute(ctx context.Context) (State, error) { return s.setMuted(ctx, false) }

// ToggleMute flips the mute flag.
func (s *Service) ToggleMute(ctx context.Context) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.setMuted(ctx, !cur.Muted)
}

// Say speaks text through the default TTS voice.
func (s *Service) Say(ctx context.Context, text string) error {
	_, err := s.r.Exec(ctx, "say", text)
	return err
}

func (s *Service) setMuted(ctx context.Context, muted bool) (State, error) {
	_, err := s.r.Exec(ctx, "osascript", "-e",
		fmt.Sprintf("set volume output muted %t", muted))
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx)
}

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
