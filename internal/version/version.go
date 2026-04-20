// Package version exposes the binary's version metadata. The values are set
// at link time via -ldflags by GoReleaser; defaults cover local `go run`.
package version

// Version, Commit, and Date are populated at link time via -ldflags by
// GoReleaser. Defaults cover local `go run`.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line version identifier, e.g. "v0.3.1 (abc1234, 2026-04-20)".
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
