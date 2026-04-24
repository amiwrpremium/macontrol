// Package capability detects what the running macOS supports so the
// Telegram layer can hide (or politely reject) features that need
// a newer OS than the host has.
//
// The package is two ideas:
//
//   - [Version] — the parsed `sw_vers -productVersion` value, with
//     a comparison helper ([Version.AtLeast]).
//   - [Features] — a flat set of bools, one per version-gated
//     capability, derived from [Version].
//
// [Detect] glues them together at boot: it shells out to
// `sw_vers`, parses the answer, derives the feature set, and
// returns a [Report] the daemon stores on bot.Deps.Capability.
// Handlers and keyboards consult that Report instead of
// hard-coding macOS-version checks; tests construct their own
// Report directly via [DeriveFeatures] or by zero-valuing the
// struct.
//
// Adding a new gate is a four-line change: add a bool field to
// [Features], wire it in [deriveFeatures], add it to [Features.count]
// for the boot summary, and gate the relevant button + handler.
package capability

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Version is the parsed form of `sw_vers -productVersion` on a
// macOS host.
//
// Lifecycle:
//   - Constructed by [parseVersion] (called from [Detect]) once at
//     daemon startup. Tests construct the value directly.
//   - Read-only thereafter; comparison via [Version.AtLeast].
//
// Field roles:
//   - Major / Minor / Patch are the dotted components, parsed
//     left-to-right with strconv.Atoi. Missing components default
//     to zero, matching `sw_vers`'s "11.0" → "11.0.0" convention.
//   - Raw is the original string. Preserved so [Version.String]
//     round-trips unusual formats (e.g. macOS 11.7.10 vs 26.0
//     conventions) verbatim instead of normalising them.
type Version struct {
	// Major is the major component, e.g. 15 for "15.3.1".
	Major int

	// Minor is the minor component, e.g. 3 for "15.3.1".
	Minor int

	// Patch is the patch component, e.g. 1 for "15.3.1".
	Patch int

	// Raw is the original `sw_vers -productVersion` string,
	// preserved for verbatim rendering by [Version.String].
	Raw string
}

// String returns the original macOS version string when [Version.Raw]
// is set, falling back to "MAJOR.MINOR.PATCH" for zero-Raw values
// (typically tests that construct a Version directly).
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// AtLeast reports whether v is greater than or equal to the
// (major, minor) pair.
//
// Behavior:
//   - Compares Major first; if Major differs, returns Major>major.
//   - Otherwise returns Minor>=minor.
//   - Patch is intentionally ignored — every version-gated feature
//     in macontrol cares about minor-level OS releases, not patch
//     fixes.
//
// Returns true when v meets the threshold.
func (v Version) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// Features enumerates every macOS-version-gated capability the bot
// cares about. Handlers and keyboards consult this struct instead
// of comparing version numbers themselves; the gate definition
// lives in [deriveFeatures] only.
//
// Lifecycle:
//   - Built once at daemon startup by [Detect] → [deriveFeatures].
//   - Stored on bot.Deps.Capability.Features and read by every
//     keyboard render and every handler entry.
//
// Field roles:
//   - Each bool corresponds to one gate. Adding a new gate means
//     adding a field here, wiring it in [deriveFeatures], and
//     bumping [Features.count] for the boot summary.
type Features struct {
	// NetworkQuality is true when the built-in `networkQuality`
	// CLI is available (shipped since macOS 12). Gates the
	// Wi-Fi → Speed test button.
	NetworkQuality bool

	// Shortcuts is true when the built-in `shortcuts` CLI is
	// available (shipped since macOS 13). Gates the
	// Tools → Run Shortcut… button + entire submenu.
	Shortcuts bool

	// WdutilInfo is true when `wdutil info` is available
	// (shipped since macOS 11; output shape varies by release).
	// Gates the Wi-Fi → Info diagnostics dump and the SSID
	// fallback used since macOS 14.4 broke
	// `networksetup -getairportnetwork`.
	WdutilInfo bool
}

// Report is the value the daemon emits at boot and that the
// /doctor command prints. Bundles the parsed [Version] with the
// derived [Features] so handlers can answer "what OS am I on" and
// "what can I do here" off a single struct.
//
// Lifecycle:
//   - Constructed by [Detect] once at daemon startup.
//   - Stored on bot.Deps.Capability and read by every handler.
//
// Field roles:
//   - Version is the parsed `sw_vers -productVersion` answer.
//   - Features is the per-capability flag set derived from Version.
type Report struct {
	// Version is the parsed macOS version the current host is
	// running.
	Version Version

	// Features is the set of version-gated capabilities derived
	// from Version.
	Features Features
}

// Detect shells out to `sw_vers -productVersion`, parses the
// answer, and returns a [Report] ready to store on
// bot.Deps.Capability.
//
// Behavior:
//   - Uses the supplied [runner.Runner] so tests can inject a
//     [runner.Fake] that returns a canned version string.
//   - On a non-Darwin host (e.g. the Linux dev box) the runner
//     fake is what supplies the answer; on a real Mac the bare
//     `sw_vers` binary returns it.
//
// Returns the [Report] and any underlying runner error wrapping
// the failed `sw_vers` invocation.
func Detect(ctx context.Context, r runner.Runner) (Report, error) {
	out, err := r.Exec(ctx, "sw_vers", "-productVersion")
	if err != nil {
		return Report{}, err
	}
	v := parseVersion(strings.TrimSpace(string(out)))
	return Report{Version: v, Features: deriveFeatures(v)}, nil
}

// ParseVersion exposes [parseVersion] for tests and for callers
// (e.g. /doctor in cmd/macontrol) that have a version string in
// hand and want the parsed form without spawning `sw_vers`.
func ParseVersion(s string) Version { return parseVersion(s) }

// parseVersion splits s on '.' and assigns Major/Minor/Patch via
// strconv.Atoi. Missing components stay zero; non-numeric components
// silently parse as zero (Atoi error swallowed) since the daemon
// must boot even on a host with an unexpected version format.
//
// Behavior:
//   - Stamps Raw with the input so [Version.String] round-trips.
//   - Treats any number of components beyond Patch as garbage and
//     ignores them.
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

// DeriveFeatures exposes [deriveFeatures] so callers (tests,
// /doctor) can compute the feature set from an arbitrary version
// string without going through [Detect]'s subprocess shell-out.
func DeriveFeatures(v Version) Features { return deriveFeatures(v) }

// deriveFeatures maps a [Version] to its [Features] flag set.
// This is the one place new gates get wired in.
func deriveFeatures(v Version) Features {
	return Features{
		NetworkQuality: v.AtLeast(12, 0),
		Shortcuts:      v.AtLeast(13, 0),
		WdutilInfo:     v.AtLeast(11, 0),
	}
}

// Summary renders a human-readable one-line boot report, e.g.
// "macOS 15.3 · 3/3 version-gated features available".
//
// Used at daemon startup (logged at INFO) and by `macontrol doctor`
// (printed to stdout).
func (r Report) Summary() string {
	available, total := r.Features.count()
	return fmt.Sprintf("macOS %s · %d/%d version-gated features available",
		r.Version, available, total)
}

// count returns (available, total) over the [Features] bool
// fields. Used by [Report.Summary] to compose the "X/Y features
// available" line; total must be bumped when a new gate field is
// added — see the smells list.
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
