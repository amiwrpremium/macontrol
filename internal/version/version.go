// Package version exposes the binary's build-time identity:
// semver tag, git short SHA, and build date. The three values
// are stamped at link time by GoReleaser via
// `-ldflags "-X internal/version.Version=… -X …Commit=… -X
// …Date=…"`; the defaults cover `go run` / `go build` without
// the ldflags.
//
// The package is small on purpose — it has no logic beyond
// holding the strings and rendering a one-liner. Callers
// (`macontrol version`, the daemon's startup log, the
// /version slash command if one is added) read the vars or
// call [String] directly.
//
// Two reasons it's a separate package rather than vars on
// cmd/macontrol:
//
//   - Anything in internal/* can read it without an import
//     cycle. The cmd package depends on internal packages,
//     not the other way around.
//   - GoReleaser can stamp via a single `-X` per var instead
//     of the awkward `cmd/macontrol.version=…` path.
package version

// Version, Commit, and Date are stamped at link time by
// GoReleaser. Local builds (`go run`, `go build` without
// the explicit -ldflags) see the safe-default sentinel
// values:
//
//   - Version = "dev"      — semver tag would normally be
//     "0.6.0" or similar.
//   - Commit  = "none"     — git short SHA would normally be
//     "a1b2c3d".
//   - Date    = "unknown"  — RFC3339 build timestamp would
//     normally be "2026-04-23T17:00:00Z".
//
// All three are package-level `var`s (not `const`) precisely
// so the linker can rewrite them. Tests may also reset them
// for "what does the version line look like in scenario X"
// fixtures.
var (
	// Version is the semver tag stamped at build time.
	Version = "dev"

	// Commit is the short git SHA stamped at build time.
	Commit = "none"

	// Date is the RFC3339 build timestamp stamped at build
	// time.
	Date = "unknown"
)

// String returns a one-line version identifier, e.g.
// "v0.6.0 (a1b2c3d, 2026-04-23T17:00:00Z)" or
// "dev (none, unknown)" for an unstamped local build.
//
// Used by the daemon's startup log and by anywhere that
// wants a single user-facing line. The cmd/macontrol
// `version` subcommand intentionally renders its own
// equivalent (with a leading "macontrol " brand) rather
// than using this — see cmd/macontrol/main.go.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
