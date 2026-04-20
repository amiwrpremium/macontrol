package version_test

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/version"
)

func TestString_ContainsAllFields(t *testing.T) {
	// These are package-level vars; reset after the test to avoid polluting
	// other packages that might read them.
	saved := [3]string{version.Version, version.Commit, version.Date}
	defer func() {
		version.Version = saved[0]
		version.Commit = saved[1]
		version.Date = saved[2]
	}()

	version.Version = "v1.2.3"
	version.Commit = "abcdef1"
	version.Date = "2026-04-20"
	got := version.String()
	for _, want := range []string{"v1.2.3", "abcdef1", "2026-04-20"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

func TestString_Defaults(t *testing.T) {
	// With the package defaults, String should still produce a parseable
	// one-liner with the sentinel strings.
	saved := [3]string{version.Version, version.Commit, version.Date}
	defer func() {
		version.Version = saved[0]
		version.Commit = saved[1]
		version.Date = saved[2]
	}()

	version.Version = "dev"
	version.Commit = "none"
	version.Date = "unknown"
	got := version.String()
	if !strings.Contains(got, "dev") || !strings.Contains(got, "none") {
		t.Fatalf("String() = %q", got)
	}
}
