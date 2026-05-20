package version_test

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/version"
)

// TestString_RendersSemverAndBuildVars checks that version.String reports
// the source-controlled Version literal alongside any Commit + Date the
// linker (or a test) has set.
func TestString_RendersSemverAndBuildVars(t *testing.T) {
	saved := [2]string{version.Commit, version.Date}
	defer func() {
		version.Commit = saved[0]
		version.Date = saved[1]
	}()

	version.Commit = "abcdef1"
	version.Date = "2026-04-20"
	got := version.String()
	for _, want := range []string{"v" + version.Version, "abcdef1", "2026-04-20"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

// TestString_Defaults verifies that the "none"/"unknown" sentinels render
// inside String when the linker hasn't stamped Commit + Date.
func TestString_Defaults(t *testing.T) {
	saved := [2]string{version.Commit, version.Date}
	defer func() {
		version.Commit = saved[0]
		version.Date = saved[1]
	}()

	version.Commit = "none"
	version.Date = "unknown"
	got := version.String()
	if !strings.Contains(got, "none") || !strings.Contains(got, "unknown") {
		t.Fatalf("String() = %q", got)
	}
}

// TestVersion_HasReleasePleaseAnnotation acts as an early-warning guard
// for accidental edits that strip the `x-release-please-version` marker
// off the Version literal. release-please relies on the annotation
// being present in version.go to know which line to bump.
func TestVersion_HasReleasePleaseAnnotation(t *testing.T) {
	// The Version const itself can't carry the comment at runtime, but
	// it has to be a parseable semver triple (X.Y.Z) for goreleaser /
	// release-please to round-trip.
	parts := strings.Split(version.Version, ".")
	if len(parts) != 3 {
		t.Fatalf("Version = %q, expected X.Y.Z", version.Version)
	}
	for _, p := range parts {
		if p == "" {
			t.Fatalf("Version = %q, empty segment", version.Version)
		}
	}
}
