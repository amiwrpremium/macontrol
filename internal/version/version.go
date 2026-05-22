// Package version exposes the binary's build-time identity:
// semver tag (release-please-managed), git short SHA, and
// build date.
//
// The semver value lives as a source-controlled constant
// annotated with the `x-release-please-version` marker; the
// release-please workflow bumps this literal on every release
// cut. The commit + date values are still stamped at link time
// by goreleaser via `-ldflags`, since they vary per build.
//
// Two reasons it's a separate package rather than vars on
// cmd/macontrol:
//
//   - Anything in internal/* can read it without an import
//     cycle. The cmd package depends on internal packages,
//     not the other way around.
//   - goreleaser can stamp via a single `-X` per var instead
//     of the awkward `cmd/macontrol.commit=…` path.
package version

// Version is the project's semantic version. The literal is
// maintained by release-please via the trailing
// `// x-release-please-version` annotation; do not edit by
// hand.
const Version = "1.0.2" // x-release-please-version

// Commit and Date are stamped at link time by goreleaser.
// Local builds (`go run`, `go build` without explicit
// -ldflags) see the safe-default sentinel values:
//
//   - Commit = "none"     — git short SHA would normally be
//     "a1b2c3d".
//   - Date   = "unknown"  — RFC3339 build timestamp would
//     normally be "2026-04-23T17:00:00Z".
//
// Both are package-level `var`s (not `const`) so the linker
// can rewrite them. Tests may also reset them for "what does
// the version line look like in scenario X" fixtures.
var (
	// Commit is the short git SHA stamped at build time.
	Commit = "none"

	// Date is the RFC3339 build timestamp stamped at build
	// time.
	Date = "unknown"
)

// String returns a one-line version identifier, e.g.
// "v0.6.0 (a1b2c3d, 2026-04-23T17:00:00Z)" or
// "v0.0.0 (none, unknown)" for an unstamped local build.
//
// Used by the daemon's startup log and by anywhere that wants
// a single user-facing line. The cmd/macontrol `version`
// subcommand intentionally renders its own equivalent (with a
// leading "macontrol " brand) rather than using this — see
// cmd/macontrol/main.go.
func String() string {
	return "v" + Version + " (" + Commit + ", " + Date + ")"
}
