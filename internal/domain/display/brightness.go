// Package display exposes brightness and screensaver control for the
// built-in display. Primary path is the `brightness` brew formula;
// screen-saver uses the built-in ScreenSaverEngine app.
package display

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// State is a snapshot of the built-in display's brightness.
type State struct {
	// Level is 0.0 to 1.0. -1 means the value is unknown (no brightness
	// tool installed and the osascript fallback can't read current level).
	Level float64
}

// Service controls the built-in display.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Get reads the current brightness. Requires `brightness` CLI.
func (s *Service) Get(ctx context.Context) (State, error) {
	out, err := s.r.Exec(ctx, "brightness", "-l")
	if err != nil {
		return State{Level: -1}, err
	}
	// Output lines look like:
	//   display 0: brightness 0.682354
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "display 0:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		level, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			return State{Level: -1}, fmt.Errorf("parse brightness: %w", err)
		}
		return State{Level: clamp01(level)}, nil
	}
	return State{Level: -1}, fmt.Errorf("display 0 not found in brightness output: %q", out)
}

// Set writes absolute brightness (0.0..1.0).
func (s *Service) Set(ctx context.Context, level float64) (State, error) {
	level = clamp01(level)
	_, err := s.r.Exec(ctx, "brightness", formatFloat(level))
	if err != nil {
		return State{Level: -1}, err
	}
	return State{Level: level}, nil
}

// Adjust changes brightness by delta, clamped to [0,1].
func (s *Service) Adjust(ctx context.Context, delta float64) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil || cur.Level < 0 {
		return cur, err
	}
	return s.Set(ctx, cur.Level+delta)
}

// Screensaver launches the screen saver (built-in, no brew dep).
func (s *Service) Screensaver(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "open", "-a", "ScreenSaverEngine")
	return err
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
