package system

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// ThermalPressure is the macOS-reported thermal pressure level
// from `powermetrics --samplers thermal`. Apple defines five
// canonical values: "Nominal" (everything's fine), "Moderate"
// (warming up), "Heavy" (throttling), "Trapping" (severe
// throttling), and "Sleeping" (system is in low-power state).
//
// Macontrol uses the literal string verbatim; the keyboard layer
// maps it to a colour-coded label via pressureLabel in handlers/sys.go.
// The sentinel string "unknown" is added by [Service.Thermal] when
// powermetrics fails or its output can't be parsed.
type ThermalPressure string

// Thermal bundles the two temperature-ish signals macontrol can
// reliably read on Apple Silicon. Returned by [Service.Thermal];
// rendered as the System → Temperature dashboard panel.
//
// Lifecycle:
//   - Constructed by Service.Thermal each time the dashboard is
//     opened or refreshed. Never cached.
//
// Field roles:
//   - Pressure is the always-populated macOS pressure level (or
//     "unknown" sentinel on failure).
//   - CPUTempC / GPUTempC are the optional °C readings from the
//     `smctemp` brew formula. Zero when smctemp is not installed
//     OR when one of the two flags failed individually.
//   - SmctempAvail is the "render the °C section?" hint —
//     true when at least one of CPUTempC or GPUTempC was read
//     successfully. Avoids the awkward "0.0°C / 0.0°C" rendering
//     when smctemp isn't available.
type Thermal struct {
	// Pressure is the macOS-reported thermal pressure level.
	// Always populated; the sentinel "unknown" is used when
	// `sudo powermetrics` failed or its output could not be
	// parsed.
	Pressure ThermalPressure

	// CPUTempC is an approximate CPU package temperature in °C
	// from `smctemp -c`. Zero when smctemp is not installed or
	// when the -c invocation failed.
	CPUTempC float64

	// GPUTempC is an approximate GPU temperature in °C from
	// `smctemp -g`. Zero when smctemp is not installed or when
	// the -g invocation failed.
	GPUTempC float64

	// SmctempAvail is true when at least one of the two smctemp
	// invocations succeeded. The renderer uses this to decide
	// whether to show the °C section at all.
	SmctempAvail bool
}

// Thermal reads a single-sample thermal snapshot by composing
// `sudo powermetrics` (for the macOS pressure level) with the
// optional `smctemp` brew formula (for °C readings).
//
// Behavior:
//  1. Pre-stamps Pressure with the "unknown" sentinel so the
//     renderer always has something to show.
//  2. Runs `sudo powermetrics -n 1 -i 1000 --samplers thermal`
//     for a single 1-second sample. The narrow sudoers entry
//     covers powermetrics specifically. On success, walks the
//     output for the "Current pressure level:" line and
//     overwrites Pressure with the parsed value.
//  3. Tries `smctemp -c` for CPU °C and `smctemp -g` for GPU °C
//     independently. Each success populates the corresponding
//     field and sets SmctempAvail=true. Each failure leaves the
//     field at zero.
//
// Returns the populated [Thermal] and a nil error today —
// powermetrics + smctemp failures are non-fatal because the
// dashboard prefers partial info over a hard error.
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

// runSmctemp shells out to `smctemp <flag>` and parses the
// trimmed stdout as a float64.
//
// Behavior:
//   - Returns the runner error verbatim when smctemp isn't on
//     $PATH (the typical "this brew formula isn't installed"
//     case) or when it exits non-zero.
//   - Returns "smctemp returned empty output" when stdout is
//     empty after trimming — defends against an installed-but-
//     misbehaving smctemp.
//   - Returns the strconv.ParseFloat error verbatim when stdout
//     is non-empty but not a valid float.
//
// Used by [Service.Thermal] for both -c (CPU) and -g (GPU)
// readings.
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
