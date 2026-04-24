// Package display exposes brightness and screensaver control
// for the built-in display.
//
// Brightness goes through the optional `brightness` brew formula
// (no built-in macOS CLI exposes the property cleanly). The
// screensaver path uses macOS's built-in ScreenSaverEngine.app
// directly so it works with no brew dependencies.
//
// CoreDisplay denial caveat: on macOS 15+ / modern Apple
// Silicon, Apple has restricted the private CoreDisplay APIs the
// `brightness` formula relies on. The CLI then exits 0 but emits
// a header + error line on stderr instead of a real value:
//
//	display 0: main, active, awake, online, built-in, ID 0x1
//	brightness: failed to get brightness of display 0x1 (error -536870201)
//
// [Service.Get] uses [runner.Runner.ExecCombined] to capture
// both streams and surfaces the tool's own error line in the
// returned error so the dashboard can render something honest.
// PR #52 was the fix for this.
package display

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// State is the read-side snapshot of the built-in display's
// brightness. Returned by [Service.Get] and by every write
// method.
//
// Lifecycle:
//   - Constructed by Service.Get on every call. Never cached.
//
// Field roles:
//   - Level is on a 0.0..1.0 scale to match the brightness
//     CLI's native domain. Sentinel -1 means "unknown" — used
//     when the CLI is missing entirely or when CoreDisplay
//     denied the read. The dashboard renders -1 as "level
//     unknown" rather than "0%".
type State struct {
	// Level is the brightness on a 0.0..1.0 scale. Sentinel
	// -1 means the value is unknown (no brightness CLI
	// installed, or CoreDisplay denied the read).
	Level float64
}

// Service is the display control surface. One instance per
// process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Display.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
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

// Get reads the current brightness via `brightness -l` and
// parses the first matching display line.
//
// Behavior:
//   - Uses [runner.Runner.ExecCombined] so stderr (where modern
//     CoreDisplay-denied builds emit the real error) is merged
//     with stdout. Plain Exec would discard stderr and the
//     dashboard would just see an empty success.
//   - Returns ({Level: -1}, err) on subprocess failure.
//   - On success, walks output line by line looking for the
//     stable "display N: brightness <float>" pattern. On
//     match, parses the float, clamps to [0, 1] via [clamp01],
//     and returns ({Level}, nil).
//   - When no display line matched (CoreDisplay denied case),
//     returns ({Level: -1}, "brightness CLI returned no
//     readable level: <first-error-line>") with the brightness
//     CLI's own diagnostic surfaced via [firstErrLine].
//
// The line filter is strict (`fields[0] == "display"` AND
// `fields[2] == "brightness"`) so the CoreDisplay-denied
// header line ("display 0: main, active, awake, online, …")
// can't be misread as a brightness value of 0.
func (s *Service) Get(ctx context.Context) (State, error) {
	// brightness writes header + error lines to stderr even when it
	// exits 0, so capture both streams or the dashboard surfaces
	// nothing useful when CoreDisplay denies the read.
	out, err := s.r.ExecCombined(ctx, "brightness", "-l")
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

// firstErrLine extracts a useful diagnostic from the brightness
// CLI's combined output for inclusion in the [Service.Get]
// "no readable level" error message.
//
// Routing rules (first match wins):
//  1. The first line starting with "brightness:" (the CLI's own
//     error format, e.g. "brightness: failed to get brightness
//     of display 0x1 (error -536870201)") → return verbatim.
//  2. No "brightness:" line, but at least one non-empty line →
//     return "got: <first-non-empty-line>", truncated to 200
//     chars with an ellipsis if longer.
//  3. Output is entirely empty (or only whitespace) → return
//     the literal "empty output".
func firstErrLine(out string) string {
	var firstLine string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "brightness:") {
			return line
		}
		if firstLine == "" {
			firstLine = line
		}
	}
	if firstLine != "" {
		const maxLen = 200
		if len(firstLine) > maxLen {
			firstLine = firstLine[:maxLen] + "…"
		}
		return "got: " + firstLine
	}
	return "empty output"
}

// Set writes an absolute brightness in 0.0..1.0, clamping
// out-of-range input via [clamp01].
//
// Behavior:
//   - Clamps level into [0, 1] before passing to the CLI.
//   - Runs `brightness <level>` (formatted via [formatFloat] to
//     3 decimal places, the precision the CLI accepts).
//   - On subprocess failure returns ({Level: -1}, err) — does
//     NOT call Get to refresh because the failure may be the
//     CoreDisplay denial that also breaks Get.
//   - On success returns ({Level: level}, nil) using the
//     post-clamp value (avoiding a Get roundtrip).
//
// Note: the post-write value is what the caller asked for, not
// what the hardware actually settled at. macOS may quantise to
// the nearest hardware step; for the dashboard's "set 80%" use
// case this is invisible.
func (s *Service) Set(ctx context.Context, level float64) (State, error) {
	level = clamp01(level)
	_, err := s.r.Exec(ctx, "brightness", formatFloat(level))
	if err != nil {
		return State{Level: -1}, err
	}
	return State{Level: level}, nil
}

// Adjust shifts the current brightness by delta (positive or
// negative), clamping the result into [0, 1]. Composes
// [Service.Get] with [Service.Set]; not atomic with concurrent
// brightness changes from other tools (TouchBar slider, the
// brightness keys) but the Telegram dispatcher serialises
// updates per chat so the race isn't observable in practice.
//
// Behavior:
//   - On Get failure, returns the current State (with Level=-1)
//     and the underlying error without calling Set.
//   - When Get succeeds with Level=-1 (CoreDisplay denied),
//     short-circuits — there's no point passing Level=-1 to
//     Set which would clamp to 0 and dim the screen.
//   - Otherwise calls Set with cur.Level + delta.
func (s *Service) Adjust(ctx context.Context, delta float64) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil || cur.Level < 0 {
		return cur, err
	}
	return s.Set(ctx, cur.Level+delta)
}

// Screensaver launches macOS's built-in ScreenSaverEngine.app.
// No brew dep, no permissions required, the screensaver
// activates immediately. Move the mouse / press a key to dismiss.
//
// Behavior:
//   - Runs `open -a ScreenSaverEngine`.
//   - Returns the runner error verbatim on failure (rare;
//     ScreenSaverEngine has shipped on every macOS release).
func (s *Service) Screensaver(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "open", "-a", "ScreenSaverEngine")
	return err
}

// clamp01 constrains v into the [0, 1] brightness range.
// Values below 0 saturate to 0; above 1 saturate to 1; in-range
// values pass through unchanged.
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

// formatFloat renders v with three decimal places — the
// precision the brightness CLI documents as accepting. Uses
// 'f' (no exponent) so values round-trip cleanly.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
