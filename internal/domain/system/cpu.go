package system

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// CPU is a parsed CPU snapshot. UserPct/SysPct/IdlePct sum to ~100
// when populated; Load1/5/15 come from `uptime`. Raw preserves the
// `top` "CPU usage:" line for fallback rendering when parsing fails.
type CPU struct {
	// UserPct is the percentage of CPU time spent in user mode.
	UserPct float64
	// SysPct is the percentage of CPU time spent in kernel mode.
	SysPct float64
	// IdlePct is the percentage of CPU time idle.
	IdlePct float64
	// Load1 is the 1-minute load average.
	Load1 float64
	// Load5 is the 5-minute load average.
	Load5 float64
	// Load15 is the 15-minute load average.
	Load15 float64
	// TopByCPU is the top 3 processes by CPU usage; nil when `ps`
	// failed.
	TopByCPU []Process
	// Raw is the original `top` "CPU usage: …" line, kept for
	// fallback display when parsing fails.
	Raw string
}

// CPU reads a parsed CPU snapshot. Best-effort: any source that
// fails leaves its fields zero / nil. Returns no error today —
// callers can decide what to render when everything is empty.
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

var cpuUsageRe = regexp.MustCompile(
	`(?i)CPU usage:\s+([\d.]+)%\s+user.*?([\d.]+)%\s+sys.*?([\d.]+)%\s+idle`)

// ParseCPUUsage extracts user/sys/idle percentages from `top`'s
// "CPU usage: 20.85% user, 16.25% sys, 62.88% idle" header line.
// Returns (0, 0, 0) on parse failure.
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
