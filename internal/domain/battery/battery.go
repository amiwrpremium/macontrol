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

const (
	StateCharging    ChargeState = "charging"
	StateDischarging ChargeState = "discharging"
	StateACFull      ChargeState = "AC"
	StateUnknown     ChargeState = "unknown"
)

// Status is a snapshot of the battery.
type Status struct {
	Percent       int         // 0..100, -1 if unknown (e.g. desktop Mac)
	State         ChargeState
	TimeRemaining string      // e.g. "3:42", "(no estimate)"; empty if N/A
	Present       bool        // false on Macs without a battery
}

// Health captures long-term condition.
type Health struct {
	CycleCount     int
	Condition      string // "Normal", "Service", …
	MaxCapacity    string // e.g. "91%" — Apple reports as percent of design
	ChargerWattage string // e.g. "70W", empty if unknown / not plugged in
}

// Service reads battery data.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// pmsetPercentRe captures percentage + state + optional time-remaining from
// lines like:
//   84%; charging; 1:02 remaining present: true
//   55%; discharging; 3:42 remaining present: true
//   100%; charged; 0:00 remaining present: true
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
