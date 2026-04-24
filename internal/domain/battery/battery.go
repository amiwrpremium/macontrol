// Package battery reads battery percentage, charging state, and
// long-term health from `pmset -g batt` and
// `system_profiler SPPowerDataType`.
//
// Two read surfaces:
//
//   - [Service.Get] returns the live percentage + charge state +
//     time-remaining as a [Status]. Cheap; runs `pmset` only.
//     Used by the Battery dashboard's main panel.
//   - [Service.GetHealth] returns the cycle count + condition +
//     max capacity as a [Health]. Slower (~1 second);
//     runs `system_profiler`. Used by the Battery → Health
//     drill-down panel.
//
// Macs without a battery (Mac mini, Mac Studio, Mac Pro) return
// [Status]{Present: false, Percent: -1, State: StateUnknown}
// from Get; the dashboard renders that as "not present (desktop
// Mac)".
package battery

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// ChargeState describes the active power state of the battery.
// Values are stable strings rather than enum-like ints so they
// round-trip through the dashboard renderer (which compares
// against the [State…] constants below) and survive JSON-style
// inspection in logs.
type ChargeState string

// Canonical [ChargeState] values. Anything pmset reports outside
// this set falls through to [StateUnknown].
const (
	// StateCharging is the battery actively gaining charge from
	// AC. pmset reports the literal "charging" token.
	StateCharging ChargeState = "charging"

	// StateDischarging is the battery actively losing charge
	// (running on battery power). pmset reports
	// "discharging".
	StateDischarging ChargeState = "discharging"

	// StateACFull is the battery on AC but not actively
	// charging — the laptop is fully charged, finishing the
	// last few percent (Apple's optimised battery-charging
	// feature), or thermally throttled. pmset reports any of
	// "charged", "finishing", "AC", or signals the AC-power
	// state via the wider 'AC Power' marker.
	StateACFull ChargeState = "AC"

	// StateUnknown is the sentinel for unparseable / unknown
	// pmset output. The dashboard renders it without a charging
	// glyph.
	StateUnknown ChargeState = "unknown"
)

// Status is the live battery snapshot returned by [Service.Get].
//
// Lifecycle:
//   - Constructed by Service.Get each time the Battery dashboard
//     opens or refreshes. Never cached.
//
// Field roles:
//   - Percent is 0..100 on Macs with a battery; sentinel -1 when
//     no battery is present.
//   - State is the parsed [ChargeState]; one of the canonical
//     constants or [StateUnknown].
//   - TimeRemaining is the human-readable string from pmset, e.g.
//     "3:42" or "(no estimate)". Empty when pmset didn't report
//     it (typical for AC-full state).
//   - Present is false on Macs without a battery; the dashboard
//     short-circuits to a "not present" panel in that case.
type Status struct {
	// Percent is the current charge level in 0..100. Sentinel -1
	// on Macs without a battery.
	Percent int

	// State is the active charging state.
	State ChargeState

	// TimeRemaining is the human-readable remaining time
	// reported by pmset, e.g. "3:42" or "(no estimate)". Empty
	// when pmset omitted it.
	TimeRemaining string

	// Present is false on Macs without a battery (Mac mini,
	// Mac Studio, Mac Pro). True otherwise.
	Present bool
}

// Health is the long-term battery condition snapshot returned by
// [Service.GetHealth].
//
// Lifecycle:
//   - Constructed by Service.GetHealth each time the user taps
//     Battery → Health. Never cached.
//
// Field roles:
//   - CycleCount is the lifetime full-charge equivalents. Apple
//     warrants Mac batteries for ~1000 cycles; Service is the
//     macOS-reported recommendation when this gets too high.
//   - Condition is the macOS-reported label, typically "Normal"
//     or "Service" (the recommendation to replace).
//   - MaxCapacity is the current capacity as a percentage of
//     the factory design capacity, e.g. "91%". The "%" is
//     included verbatim from system_profiler.
//   - ChargerWattage is the rated output of the attached
//     charger, e.g. "70W". Empty when no charger is plugged in
//     or system_profiler couldn't query it.
//
// Empty fields mean system_profiler didn't expose that line on
// this Mac — partial parses are normal.
type Health struct {
	// CycleCount is the lifetime charge-cycle count.
	CycleCount int

	// Condition is the macOS-reported health label, typically
	// "Normal" or "Service".
	Condition string

	// MaxCapacity is the current capacity as a percentage of
	// the factory design capacity, e.g. "91%". The "%" is
	// included verbatim.
	MaxCapacity string

	// ChargerWattage is the attached charger's rated output,
	// e.g. "70W". Empty when no charger is plugged in.
	ChargerWattage string
}

// Service is the battery read surface. One instance per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Battery.
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

// pmsetPercentRe captures the percentage + state + optional
// time-remaining tail from a pmset battery line.
//
// Pattern shape (with examples):
//
//	84%; charging; 1:02 remaining present: true
//	55%; discharging; 3:42 remaining present: true
//	100%; charged; 0:00 remaining present: true
//
// The third group is optional because some states (e.g.
// "charged") may omit the time-remaining segment on certain
// macOS releases.
var pmsetPercentRe = regexp.MustCompile(`(\d+)%;\s*([^;]+?)(?:;\s*([^;]*))?$`)

// Get reads the current battery [Status] via `pmset -g batt`.
//
// Behavior:
//  1. Shells out to `pmset -g batt`. Returns ({}, err) on
//     subprocess failure.
//  2. Detects the no-battery sentinels ("Battery is not
//     present" / "No batteries available") in the full output;
//     returns Status{Present: false, Percent: -1, State:
//     StateUnknown} on match.
//  3. Walks the output line by line looking for a percentage
//     match via [pmsetPercentRe]. On match, parses the
//     percentage int, the state via [parseChargeState], and
//     the optional time-remaining string. Returns a populated
//     [Status] with Present=true.
//  4. Returns ({}, "unrecognized pmset output: %q") when no
//     line matched the regex (defensive — pmset output should
//     always include at least one such line on a Mac with a
//     battery).
//
// The TimeRemaining tail is post-processed to strip the
// trailing " present: true" suffix pmset appends on some macOS
// versions.
func (s *Service) Get(ctx context.Context) (Status, error) {
	out, err := s.r.Exec(ctx, "pmset", "-g", "batt")
	if err != nil {
		return Status{}, err
	}
	text := string(out)
	if strings.Contains(text, "Battery is not present") ||
		strings.Contains(text, "No batteries available") {
		return Status{Present: false, Percent: -1, State: StateUnknown}, nil
	}
	for _, line := range strings.Split(text, "\n") {
		m := pmsetPercentRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pct, _ := strconv.Atoi(m[1])
		st := parseChargeState(m[2], text)
		rem := ""
		if len(m) > 3 {
			rem = strings.TrimSpace(m[3])
			// Strip trailing " present: true" if pmset appended it.
			rem = strings.TrimSuffix(rem, " present: true")
			rem = strings.TrimSpace(rem)
		}
		return Status{
			Percent:       pct,
			State:         st,
			TimeRemaining: rem,
			Present:       true,
		}, nil
	}
	return Status{}, fmt.Errorf("unrecognized pmset output: %q", text)
}

// GetHealth returns the long-term battery [Health] via
// `system_profiler SPPowerDataType`.
//
// Behavior:
//   - Shells out (~1 second; slow because system_profiler walks
//     several IOKit subsystems). Returns ({}, err) on
//     subprocess failure.
//   - Walks output line by line looking for the four KEY:
//     prefixes (Cycle Count, Condition, Maximum Capacity,
//     Wattage (W)). Each match is trimmed and stamped onto the
//     corresponding [Health] field.
//   - Wattage gets a "W" suffix appended for display
//     consistency ("70W").
//   - Lines not matching any of the four prefixes are silently
//     ignored.
//
// Returns the populated [Health] on success; partial parses are
// normal (a Mac without a charger plugged in won't have the
// Wattage line).
func (s *Service) GetHealth(ctx context.Context) (Health, error) {
	out, err := s.r.Exec(ctx, "system_profiler", "SPPowerDataType")
	if err != nil {
		return Health{}, err
	}
	h := Health{}
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Cycle Count:"):
			h.CycleCount, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Cycle Count:")))
		case strings.HasPrefix(line, "Condition:"):
			h.Condition = strings.TrimSpace(strings.TrimPrefix(line, "Condition:"))
		case strings.HasPrefix(line, "Maximum Capacity:"):
			h.MaxCapacity = strings.TrimSpace(strings.TrimPrefix(line, "Maximum Capacity:"))
		case strings.HasPrefix(line, "Wattage (W):"):
			h.ChargerWattage = strings.TrimSpace(strings.TrimPrefix(line, "Wattage (W):")) + "W"
		}
	}
	return h, nil
}

// parseChargeState maps a pmset state token (the trimmed second
// regex group) to a canonical [ChargeState] constant.
//
// Routing rules (first match wins):
//  1. token == "discharging" → [StateDischarging].
//  2. token == "charging" → [StateCharging].
//  3. token ∈ {"charged", "finishing", "ac"} → [StateACFull].
//     ("finishing" is Apple's optimised-battery-charging label
//     for the slow trickle from ~80% → 100%.)
//  4. Token didn't match exactly, but the wider pmset output
//     contains the literal "'AC Power'" marker → [StateACFull]
//     as a fallback signal that the charger is plugged in.
//  5. Otherwise → [StateUnknown].
//
// Token comparison is case-insensitive and tolerates a trailing
// ';' (some pmset variants emit "charging;" with the trailing
// semicolon attached to the token).
func parseChargeState(token, full string) ChargeState {
	token = strings.TrimSuffix(strings.TrimSpace(token), ";")
	switch strings.ToLower(token) {
	case "discharging":
		return StateDischarging
	case "charging":
		return StateCharging
	case "charged", "finishing", "ac":
		return StateACFull
	}
	if strings.Contains(full, "'AC Power'") {
		return StateACFull
	}
	return StateUnknown
}
