package system

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// ThermalPressure reflects macOS-reported pressure level, one of:
// "Nominal", "Moderate", "Heavy", "Trapping", "Sleeping".
type ThermalPressure string

// Thermal bundles the two temperature-ish signals we can reliably read on
// Apple Silicon: (1) macOS thermal pressure from powermetrics (requires sudo
// for the narrow sudoers entry), and (2) an approximate °C reading from the
// optional `smctemp` brew formula.
type Thermal struct {
	Pressure     ThermalPressure // always populated; "unknown" on failure
	CPUTempC     float64         // 0 if smctemp not installed
	GPUTempC     float64         // 0 if smctemp not installed
	SmctempAvail bool            // true if smctemp readings succeeded
}

// Thermal reads a single-sample thermal snapshot.
func (s *Service) Thermal(ctx context.Context) (Thermal, error) {
	t := Thermal{Pressure: "unknown"}

	if out, err := s.r.Sudo(ctx, "powermetrics", "-n", "1", "-i", "1000", "--samplers", "thermal"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Current pressure level:") {
				t.Pressure = ThermalPressure(strings.TrimSpace(strings.TrimPrefix(line, "Current pressure level:")))
				break
			}
		}
	}
	// smctemp is optional.
	if cpu, err := s.runSmctemp(ctx, "-c"); err == nil {
		t.CPUTempC = cpu
		t.SmctempAvail = true
	}
	if gpu, err := s.runSmctemp(ctx, "-g"); err == nil {
		t.GPUTempC = gpu
		t.SmctempAvail = true
	}
	return t, nil
}

func (s *Service) runSmctemp(ctx context.Context, flag string) (float64, error) {
	out, err := s.r.Exec(ctx, "smctemp", flag)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, errors.New("smctemp returned empty output")
	}
	return strconv.ParseFloat(trimmed, 64)
}
