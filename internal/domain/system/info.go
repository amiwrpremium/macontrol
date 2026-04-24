package system

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Uptime is the parsed view of the macOS `uptime` command. Returned
// as a field of [Info]; also rendered standalone by the System
// dashboard.
//
// Lifecycle:
//   - Constructed by [parseUptime] each time [Service.Info] runs.
//     Never cached.
//
// Field roles:
//   - Duration is the human-friendly uptime string with bare
//     "HH:MM" segments rewritten to "Xh Ym".
//   - Users / Load1 / Load5 / Load15 are extracted via the
//     regexes in this file. Any field that can't be parsed
//     stays at its zero value; the renderer is responsible for
//     deciding whether to show "0 users" or hide the row.
//   - Raw is the unmodified `uptime` line. Always populated so
//     handlers can fall back to verbatim display when parsing
//     missed a field.
type Uptime struct {
	// Duration is a human-readable uptime string, e.g.
	// "3 days, 6h 27m". Bare "HH:MM" segments from the raw
	// `uptime` line are rewritten to "Xh Ym" by
	// [prettyUptimeDuration].
	Duration string

	// Users is the count of logged-in users from the `uptime`
	// line. Zero when unparseable.
	Users int

	// Load1 is the 1-minute load average. Zero when unparseable.
	Load1 float64

	// Load5 is the 5-minute load average. Zero when unparseable.
	Load5 float64

	// Load15 is the 15-minute load average. Zero when
	// unparseable.
	Load15 float64

	// Raw is the original `uptime` line, always populated so
	// callers can fall back to verbatim rendering when parsing
	// misses a field.
	Raw string
}

// Info is a coarse OS + hardware summary suitable for the System →
// Info dashboard. Composed by [Service.Info] from a handful of
// macOS CLIs; every field is best-effort.
//
// Lifecycle:
//   - Constructed once per [Service.Info] call. Never cached.
//
// Field roles:
//   - ProductName / ProductVersion / BuildVersion come from
//     `sw_vers` (the same source [capability.Detect] reads).
//   - Hostname comes from `hostname`.
//   - Model / ChipName / TotalRAMBytes come from `sysctl`.
//   - CPUCores comes from `system_profiler SPHardwareDataType`
//     because sysctl reports just the integer count, not the
//     "X performance + Y efficiency" breakdown.
//   - Uptime is the parsed [Uptime] snapshot.
//   - Any individual subprocess failure leaves its field empty
//     / zero — Info itself never returns an error today (see
//     the smells list).
type Info struct {
	// ProductName is the OS marketing name from `sw_vers`,
	// typically "macOS".
	ProductName string

	// ProductVersion is the dotted OS version from `sw_vers`,
	// e.g. "15.3.1".
	ProductVersion string

	// BuildVersion is the macOS build identifier from `sw_vers`,
	// e.g. "24D70".
	BuildVersion string

	// Hostname is the kernel hostname from the `hostname`
	// command (equivalent to scutil --get LocalHostName for the
	// default user).
	Hostname string

	// Model is the hardware identifier from
	// `sysctl hw.model`, e.g. "MacBookPro18,3".
	Model string

	// ChipName is the CPU brand string from
	// `sysctl machdep.cpu.brand_string`, e.g. "Apple M3 Pro".
	ChipName string

	// CPUCores is the human-readable core breakdown from
	// `system_profiler SPHardwareDataType`, e.g.
	// "11 (6 performance and 5 efficiency)".
	CPUCores string

	// TotalRAMBytes is `sysctl hw.memsize` parsed as a uint64.
	// Zero when the read failed.
	TotalRAMBytes uint64

	// Uptime is the parsed `uptime` snapshot.
	Uptime Uptime
}

// Service is the system-info read surface. Bundles every
// system-related operation behind the [runner.Runner] dependency
// so tests can inject a [runner.Fake].
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.System.
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

// Info reads the OS + hardware summary by composing several
// macOS CLI calls, best-effort.
//
// Behavior:
//  1. Reads `sw_vers` and parses each "Key: Value" line for
//     ProductName / ProductVersion / BuildVersion.
//  2. Reads `hostname` for the trimmed Hostname.
//  3. Reads `sysctl -n hw.model`, `sysctl -n
//     machdep.cpu.brand_string`, and `sysctl -n hw.memsize` for
//     Model / ChipName / TotalRAMBytes.
//  4. Reads `uptime` and pipes through [parseUptime] for the
//     Uptime snapshot.
//  5. Reads `system_profiler SPHardwareDataType` and extracts
//     the "Total Number of Cores: …" line for CPUCores.
//
// Each subprocess failure is silently ignored — the returned
// Info just has that field at its zero value. The error return
// is currently always nil (see the smells list); callers should
// not branch on it.
func (s *Service) Info(ctx context.Context) (Info, error) {
	i := Info{}
	s.fillSwVers(ctx, &i)
	s.fillHostname(ctx, &i)
	s.fillSysctl(ctx, &i)
	s.fillUptime(ctx, &i)
	s.fillCPUCores(ctx, &i)
	return i, nil
}

// swVersSetters maps each sw_vers KEY to the [Info] field it
// fills. Built once at init.
var swVersSetters = map[string]func(*Info, string){
	"ProductName":    func(i *Info, v string) { i.ProductName = v },
	"ProductVersion": func(i *Info, v string) { i.ProductVersion = v },
	"BuildVersion":   func(i *Info, v string) { i.BuildVersion = v },
}

func (s *Service) fillSwVers(ctx context.Context, i *Info) {
	out, err := s.r.Exec(ctx, "sw_vers")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := splitKV(line)
		if !ok {
			continue
		}
		if setter, ok := swVersSetters[k]; ok {
			setter(i, v)
		}
	}
}

func (s *Service) fillHostname(ctx context.Context, i *Info) {
	if out, err := s.r.Exec(ctx, "hostname"); err == nil {
		i.Hostname = strings.TrimSpace(string(out))
	}
}

func (s *Service) fillSysctl(ctx context.Context, i *Info) {
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.model"); err == nil {
		i.Model = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
		i.ChipName = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.memsize"); err == nil {
		_, _ = fmt.Sscan(strings.TrimSpace(string(out)), &i.TotalRAMBytes)
	}
}

func (s *Service) fillUptime(ctx context.Context, i *Info) {
	if out, err := s.r.Exec(ctx, "uptime"); err == nil {
		i.Uptime = parseUptime(strings.TrimSpace(string(out)))
	}
}

func (s *Service) fillCPUCores(ctx context.Context, i *Info) {
	out, err := s.r.Exec(ctx, "system_profiler", "SPHardwareDataType")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Total Number of Cores:") {
			i.CPUCores = strings.TrimSpace(strings.TrimPrefix(line, "Total Number of Cores:"))
		}
	}
}

// splitKV splits a "Key: Value" line on the first ':' and trims
// whitespace from both halves. Returns ok=false when the line has
// no ':' at all. Used by [Service.Info]'s sw_vers parser.
func splitKV(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// Compiled regexes used by [parseUptime]:
//
//   - upUserRe captures the duration token between "up " and ", N
//     users" plus the user count itself.
//   - loadRe captures the three load-average floats; macOS
//     occasionally writes them comma-separated, occasionally
//     space-separated, so the alternation [,\s] handles both.
//   - hhmmRe finds bare "HH:MM" tokens inside the duration so
//     [prettyUptimeDuration] can rewrite them.
var (
	// upUserRe matches the "up …, N users" segment of `uptime`.
	upUserRe = regexp.MustCompile(`(?i)up\s+(.*?),\s*(\d+)\s+users?\b`)

	// loadRe matches the "load averages: 1.23, 4.56, 7.89"
	// segment of `uptime`. macOS writes either commas or spaces
	// between the three floats — the [,\s] alternation accepts
	// both.
	loadRe = regexp.MustCompile(`(?i)load\s+averages?:\s+([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)`)

	// hhmmRe matches bare "HH:MM" tokens used by [prettyUptimeDuration]
	// to rewrite them as "Xh Ym".
	hhmmRe = regexp.MustCompile(`\b(\d+):(\d+)\b`)
)

// parseUptime parses the raw output of the `uptime` command into
// a structured [Uptime] value.
//
// Behavior:
//   - Always sets Raw to the input string so callers can fall
//     back even on total parse failure.
//   - Runs upUserRe against raw to extract the duration token
//     and user count. On match, the duration is normalised via
//     [prettyUptimeDuration].
//   - Runs loadRe against raw to extract the three load
//     averages. ParseFloat errors are silently swallowed (each
//     remaining at zero).
//   - Returns even when nothing parsed; zero fields signal "not
//     present" to the renderer.
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

// prettyUptimeDuration rewrites every bare "HH:MM" token inside s
// as "Xh Ym" so durations like "3 days, 6:27" render as
// "3 days, 6h 27m" in the dashboard.
//
// Behavior:
//   - Matches via [hhmmRe] (any colon-separated digit pair word-
//     bounded).
//   - Replacement is unconditional — there's no attempt to detect
//     timestamps that aren't actually durations. In practice
//     `uptime` only emits HH:MM in duration context so this is
//     safe.
func prettyUptimeDuration(s string) string {
	return hhmmRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := strings.SplitN(m, ":", 2)
		h, _ := strconv.Atoi(parts[0])
		mn, _ := strconv.Atoi(parts[1])
		return fmt.Sprintf("%dh %dm", h, mn)
	})
}

// FirstInt returns the first contiguous run of digits in s parsed
// as an int. Used by the System renderer to pull the total core
// count out of CPUCores strings like
// "12 (8 performance and 4 efficiency)".
//
// Behavior:
//   - Walks runes left-to-right, marking the first digit's index.
//   - Stops at the first non-digit after the run starts and
//     parses [start:i] via strconv.Atoi.
//   - When the digit run reaches the end of s, parses
//     [start:len(s)].
//   - Returns (0, false) when no digit run is found or the run
//     is unparseable as an int.
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
