// Package battery reads battery percentage, charging state, and health from
// `pmset -g batt` and `system_profiler SPPowerDataType`.
package battery

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// ChargeState describes whether the battery is charging, discharging, or
// on AC but not charging (fully charged or throttled).
type ChargeState string

// ChargeState values reported by pmset.
const (
	StateCharging    ChargeState = "charging"
	StateDischarging ChargeState = "discharging"
	StateACFull      ChargeState = "AC"
	StateUnknown     ChargeState = "unknown"
)

// Status is a snapshot of the battery.
type Status struct {
	// Percent is the current charge level in the 0..100 range; -1
	// on hardware without a battery (Mac mini, Mac Studio).
	Percent int
	// State is the active charging state as parsed from pmset.
	State ChargeState
	// TimeRemaining is the human-readable remaining time, e.g.
	// "3:42" or "(no estimate)"; empty when pmset did not report
	// one.
	TimeRemaining string
	// Present is false on Macs without a battery.
	Present bool
}

// Health captures long-term condition.
type Health struct {
	// CycleCount is the lifetime charge-cycle count.
	CycleCount int
	// Condition is the macOS-reported health label, e.g. "Normal"
	// or "Service".
	Condition string
	// MaxCapacity is the current capacity as a percentage of the
	// factory design capacity, e.g. "91%".
	MaxCapacity string
	// ChargerWattage is the attached charger's rated output, e.g.
	// "70W"; empty when unknown or not plugged in.
	ChargerWattage string
}

// Service reads battery data.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// pmsetPercentRe captures percentage + state + optional time-remaining from
// lines like:
//
//	84%; charging; 1:02 remaining present: true
//	55%; discharging; 3:42 remaining present: true
//	100%; charged; 0:00 remaining present: true
var pmsetPercentRe = regexp.MustCompile(`(\d+)%;\s*([^;]+?)(?:;\s*([^;]*))?$`)

// Get reads current battery status.
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

// GetHealth returns long-term battery condition.
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
