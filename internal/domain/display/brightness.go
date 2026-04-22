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
//
// The tool's success format is one line per display:
//
//	display 0: brightness 0.682354
//
// On macOS 15+ / modern Apple Silicon, CoreDisplay's private API is
// often denied and the tool exits 0 but emits a header + error line:
//
//	display 0: main, active, awake, online, built-in, ID 0x1
//	brightness: failed to get brightness of display 0x1 (error -536870201)
//
// The parser matches only `display <N>: brightness <float>` so the
// header line can't be mistaken for a value, and surfaces the tool's
// own error line in the returned error so the dashboard can render
// something honest.
func (s *Service) Get(ctx context.Context) (State, error) {
	out, err := s.r.Exec(ctx, "brightness", "-l")
	if err != nil {
		return State{Level: -1}, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 || fields[0] != "display" || fields[2] != "brightness" {
			continue
		}
		level, perr := strconv.ParseFloat(fields[3], 64)
		if perr != nil {
			continue
		}
		return State{Level: clamp01(level)}, nil
	}
	return State{Level: -1}, fmt.Errorf("brightness CLI returned no readable level: %s", firstErrLine(string(out)))
}

// firstErrLine returns the first line that looks like the brightness
// tool's own error output (`brightness: …`). Falls back to a generic
// message if none is present.
func firstErrLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "brightness:") {
			return line
		}
	}
	return "no `display N: brightness <value>` line in output"
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
