package system

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Uptime is a parsed view of the `uptime` command. Raw is always
// populated; the rest are zero-valued when parsing failed so the
// renderer can fall back gracefully.
type Uptime struct {
	// Duration is a human-readable uptime string, e.g. "3 days,
	// 6h 27m"; bare "HH:MM" segments from macOS are rewritten as
	// "Xh Ym" for readability.
	Duration string
	// Users is the logged-in user count from `uptime`; 0 if the
	// line could not be parsed.
	Users int
	// Load1 is the 1-minute load average; 0 if unparseable.
	Load1 float64
	// Load5 is the 5-minute load average.
	Load5 float64
	// Load15 is the 15-minute load average.
	Load15 float64
	// Raw is the original `uptime` line. Always populated so the
	// renderer can fall back when parsing misses a field.
	Raw string
}

// Info is a coarse hardware + OS summary.
type Info struct {
	// ProductName is the marketing OS name from `sw_vers`, e.g. "macOS".
	ProductName string
	// ProductVersion is the semver-style OS version, e.g. "15.3.1".
	ProductVersion string
	// BuildVersion is the macOS build identifier, e.g. "24D70".
	BuildVersion string
	// Hostname is the kernel hostname from the `hostname` command.
	Hostname string
	// Model is the hardware identifier from `sysctl hw.model`, e.g.
	// "MacBookPro18,3".
	Model string
	// ChipName is `sysctl machdep.cpu.brand_string`, e.g. "Apple M3 Pro".
	ChipName string
	// CPUCores is the human-readable core breakdown from
	// `system_profiler SPHardwareDataType`, e.g.
	// "11 (6 performance and 5 efficiency)".
	CPUCores string
	// TotalRAMBytes is `sysctl hw.memsize`; 0 when the read failed.
	TotalRAMBytes uint64
	// Uptime is the parsed uptime snapshot.
	Uptime Uptime
}

// Service bundles all read-only system helpers.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Info reads an aggregate info snapshot.
func (s *Service) Info(ctx context.Context) (Info, error) {
	i := Info{}

	if out, err := s.r.Exec(ctx, "sw_vers"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			k, v, ok := splitKV(line)
			if !ok {
				continue
			}
			switch k {
			case "ProductName":
				i.ProductName = v
			case "ProductVersion":
				i.ProductVersion = v
			case "BuildVersion":
				i.BuildVersion = v
			}
		}
	}
	if out, err := s.r.Exec(ctx, "hostname"); err == nil {
		i.Hostname = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.model"); err == nil {
		i.Model = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
		i.ChipName = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.memsize"); err == nil {
		_, _ = fmt.Sscan(strings.TrimSpace(string(out)), &i.TotalRAMBytes)
	}
	if out, err := s.r.Exec(ctx, "uptime"); err == nil {
		i.Uptime = parseUptime(strings.TrimSpace(string(out)))
	}
	// Best-effort CPU core breakdown from system_profiler.
	if out, err := s.r.Exec(ctx, "system_profiler", "SPHardwareDataType"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Total Number of Cores:") {
				i.CPUCores = strings.TrimSpace(strings.TrimPrefix(line, "Total Number of Cores:"))
			}
		}
	}
	return i, nil
}

func splitKV(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

var (
	upUserRe = regexp.MustCompile(`(?i)up\s+(.*?),\s*(\d+)\s+users?\b`)
	loadRe   = regexp.MustCompile(`(?i)load\s+averages?:\s+([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)`)
	hhmmRe   = regexp.MustCompile(`\b(\d+):(\d+)\b`)
)

// parseUptime extracts duration / users / 1-5-15 load averages from
// the raw `uptime` line. Best-effort: any field that can't be
// parsed is left at its zero value, but Raw always carries the
// original string so callers can fall back.
func parseUptime(raw string) Uptime {
	u := Uptime{Raw: raw}
	if m := upUserRe.FindStringSubmatch(raw); m != nil {
		u.Duration = prettyUptimeDuration(strings.TrimSpace(m[1]))
		if n, err := strconv.Atoi(m[2]); err == nil {
			u.Users = n
		}
	}
	if m := loadRe.FindStringSubmatch(raw); m != nil {
		u.Load1, _ = strconv.ParseFloat(m[1], 64)
		u.Load5, _ = strconv.ParseFloat(m[2], 64)
		u.Load15, _ = strconv.ParseFloat(m[3], 64)
	}
	return u
}

// prettyUptimeDuration rewrites bare HH:MM segments inside an uptime
// duration string as "Xh Ym" so "3 days, 6:27" reads as
// "3 days, 6h 27m".
func prettyUptimeDuration(s string) string {
	return hhmmRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := strings.SplitN(m, ":", 2)
		h, _ := strconv.Atoi(parts[0])
		mn, _ := strconv.Atoi(parts[1])
		return fmt.Sprintf("%dh %dm", h, mn)
	})
}

// FirstInt returns the first run of digits in s parsed as an int.
// Used by the renderer to pull the total core count out of CPUCores
// strings like "12 (8 performance and 4 efficiency)".
func FirstInt(s string) (int, bool) {
	start := -1
	for i, r := range s {
		if r >= '0' && r <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			n, err := strconv.Atoi(s[start:i])
			return n, err == nil
		}
	}
	if start >= 0 {
		n, err := strconv.Atoi(s[start:])
		return n, err == nil
	}
	return 0, false
}
