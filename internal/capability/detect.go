// Package capability detects what the current macOS release supports, so the
// Telegram layer can hide (or mark unavailable) features that require a
// newer OS.
package capability

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Version is a parsed `sw_vers -productVersion`.
type Version struct {
	// Major is the major component, e.g. 15 for "15.3.1".
	Major int
	// Minor is the minor component, e.g. 3 for "15.3.1".
	Minor int
	// Patch is the patch component, e.g. 1 for "15.3.1".
	Patch int
	// Raw is the original `sw_vers -productVersion` string, used
	// verbatim when rendering so unusual formats round-trip.
	Raw string
}

// String returns the original macOS version string when available, and
// falls back to "MAJOR.MINOR.PATCH" otherwise.
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// AtLeast reports whether v is >= (major, minor).
func (v Version) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// Features enumerates every capability whose availability depends on the
// macOS version. Handlers consult this set instead of hard-coding checks.
type Features struct {
	// NetworkQuality reports availability of the built-in
	// `networkQuality` speedtest, shipped since macOS 12.
	NetworkQuality bool
	// Shortcuts reports availability of the `shortcuts` CLI,
	// shipped since macOS 13.
	Shortcuts bool
	// WdutilInfo reports availability of `wdutil info`, shipped
	// since macOS 11 (the exact output shape still varies).
	WdutilInfo bool
}

// Report is the value the bot emits on boot and that /doctor prints.
type Report struct {
	// Version is the parsed macOS version the current host is running.
	Version Version
	// Features is the set of version-gated capabilities derived from
	// Version.
	Features Features
}

// Detect runs `sw_vers -productVersion` and returns the derived Report.
// Non-darwin hosts (e.g. the Linux dev box running tests) get a zero Version
// and an empty feature set — capability-aware code then degrades gracefully.
func Detect(ctx context.Context, r runner.Runner) (Report, error) {
	out, err := r.Exec(ctx, "sw_vers", "-productVersion")
	if err != nil {
		return Report{}, err
	}
	v := parseVersion(strings.TrimSpace(string(out)))
	return Report{Version: v, Features: deriveFeatures(v)}, nil
}

// ParseVersion is exported for tests.
func ParseVersion(s string) Version { return parseVersion(s) }

func parseVersion(s string) Version {
	v := Version{Raw: s}
	parts := strings.Split(s, ".")
	if len(parts) > 0 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		v.Patch, _ = strconv.Atoi(parts[2])
	}
	return v
}

// DeriveFeatures is exported so callers (tests, /doctor) can compute the
// feature set from an arbitrary version string.
func DeriveFeatures(v Version) Features { return deriveFeatures(v) }

func deriveFeatures(v Version) Features {
	return Features{
		NetworkQuality: v.AtLeast(12, 0),
		Shortcuts:      v.AtLeast(13, 0),
		WdutilInfo:     v.AtLeast(11, 0),
	}
}

// Summary returns a human-readable one-line boot report, e.g.
// "macOS 15.3 · 3/3 version-gated features available".
func (r Report) Summary() string {
	available, total := r.Features.count()
	return fmt.Sprintf("macOS %s · %d/%d version-gated features available",
		r.Version, available, total)
}

func (f Features) count() (available, total int) {
	flags := []bool{f.NetworkQuality, f.Shortcuts, f.WdutilInfo}
	total = len(flags)
	for _, v := range flags {
		if v {
			available++
		}
	}
	return
}
