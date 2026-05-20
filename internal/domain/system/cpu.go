package system

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// CPU is a parsed CPU snapshot composed by [Service.CPU]. The
// dashboard renders the percentage triple, the three load
// averages, and the top-3 process list as separate sections.
//
// Lifecycle:
//   - Constructed by Service.CPU each time the System → CPU
//     dashboard is opened or refreshed. Never cached.
//
// Field roles:
//   - UserPct / SysPct / IdlePct sum to ~100 when populated; they
//     come from the "CPU usage:" header line of `top -l 1`.
//   - Load1 / Load5 / Load15 come from `uptime` (same source as
//     [Uptime]), pulled separately so the CPU snapshot is
//     self-contained.
//   - TopByCPU is the top-3 processes by %CPU from
//     [Service.TopN]. nil on failure (rare); empty slice never
//     happens because TopN returns nil when ps fails.
//   - Raw is the verbatim "CPU usage:" line from `top` so the
//     renderer can fall back to it when the percentages parsed
//     to zero.
type CPU struct {
	// UserPct is the percentage of CPU time spent in user mode.
	UserPct float64

	// SysPct is the percentage of CPU time spent in kernel mode.
	SysPct float64

	// IdlePct is the percentage of CPU time idle.
	IdlePct float64

	// Load1 is the 1-minute load average from `uptime`.
	Load1 float64

	// Load5 is the 5-minute load average.
	Load5 float64

	// Load15 is the 15-minute load average.
	Load15 float64

	// TopByCPU is the top-3 processes by %CPU as returned by
	// [Service.TopN]. nil when ps failed.
	TopByCPU []Process

	// Raw is the original "CPU usage: …" line from `top`,
	// preserved so the renderer can fall back when the
	// percentage parse missed all three fields.
	Raw string
}

// CPU reads the CPU snapshot by composing several macOS CLIs.
//
// Behavior:
//  1. Reads `uptime` and pipes through [parseUptime] to extract
//     the three load averages onto Load1/Load5/Load15. Subprocess
//     failures leave those at zero.
//  2. Reads `top -l 1 -s 0` and walks for the "CPU usage:" line;
//     when found, stamps Raw and parses User/Sys/Idle via
//     [ParseCPUUsage]. The -l 1 -s 0 flags request a single
//     sample with zero seconds between samples (no warm-up).
//  3. Calls [Service.TopN] for the top-3 processes by CPU.
//
// Each subprocess failure is silently ignored — the returned CPU
// just has that section's fields at their zero / nil value. The
// error return is currently always nil (see the smells list).
func (s *Service) CPU(ctx context.Context) (CPU, error) {
	c := CPU{}
	if out, err := s.r.Exec(ctx, "uptime"); err == nil {
		u := parseUptime(strings.TrimSpace(string(out)))
		c.Load1, c.Load5, c.Load15 = u.Load1, u.Load5, u.Load15
	}
	if out, err := s.r.Exec(ctx, "top", "-l", "1", "-s", "0"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "CPU usage:") {
				continue
			}
			c.Raw = line
			c.UserPct, c.SysPct, c.IdlePct = ParseCPUUsage(line)
			break
		}
	}
	if procs, err := s.TopN(ctx, 3); err == nil && len(procs) > 0 {
		c.TopByCPU = procs
	}
	return c, nil
}

// cpuUsageRe matches the `top` "CPU usage:" header line.
//
// Pattern shape: "CPU usage: 20.85% user, 16.25% sys, 62.88% idle".
// The .*? between the three percentage groups absorbs the
// commas and labels so we don't need to special-case macOS
// formatting variants ("user," vs "user ," etc.).
var cpuUsageRe = regexp.MustCompile(
	`(?i)CPU usage:\s+([\d.]+)%\s+user.*?([\d.]+)%\s+sys.*?([\d.]+)%\s+idle`)

// ParseCPUUsage extracts the user/sys/idle percentage triple from
// a `top` "CPU usage: …" header line.
//
// Behavior:
//   - On regex match, parses each captured group via
//     strconv.ParseFloat. Parse errors are silently swallowed
//     (each remaining at zero).
//   - On no match, returns (0, 0, 0) so callers can detect
//     parse failure by summing the result and comparing to
//     ~100.
//
// Exported for tests and for any future caller that has a `top`
// line in hand and wants the parsed triple.
func ParseCPUUsage(line string) (user, sys, idle float64) {
	m := cpuUsageRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	user, _ = strconv.ParseFloat(m[1], 64)
	sys, _ = strconv.ParseFloat(m[2], 64)
	idle, _ = strconv.ParseFloat(m[3], 64)
	return
}
