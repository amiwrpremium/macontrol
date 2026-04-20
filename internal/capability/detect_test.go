package capability_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want capability.Version
	}{
		{"11.0", capability.Version{Major: 11, Minor: 0, Patch: 0, Raw: "11.0"}},
		{"14.4.1", capability.Version{Major: 14, Minor: 4, Patch: 1, Raw: "14.4.1"}},
		{"16", capability.Version{Major: 16, Minor: 0, Patch: 0, Raw: "16"}},
		{"", capability.Version{Raw: ""}},
		{"garbage", capability.Version{Raw: "garbage"}},
	}
	for _, c := range cases {
		if got := capability.ParseVersion(c.in); got != c.want {
			t.Errorf("ParseVersion(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()
	v := capability.ParseVersion("13.4.1")
	cases := []struct {
		major, minor int
		want         bool
	}{
		{13, 0, true},
		{13, 4, true},
		{13, 5, false},
		{12, 99, true}, // major wins
		{14, 0, false},
		{11, 0, true},
	}
	for _, c := range cases {
		if got := v.AtLeast(c.major, c.minor); got != c.want {
			t.Errorf("%s.AtLeast(%d,%d) = %v, want %v", v, c.major, c.minor, got, c.want)
		}
	}
}

func TestVersion_String(t *testing.T) {
	t.Parallel()
	// Non-empty Raw takes precedence.
	v := capability.Version{Major: 15, Minor: 2, Raw: "15.2"}
	if v.String() != "15.2" {
		t.Errorf("with raw: %q", v.String())
	}
	// Empty Raw falls back to numeric form.
	v2 := capability.Version{Major: 15, Minor: 2, Patch: 1}
	if v2.String() != "15.2.1" {
		t.Errorf("without raw: %q", v2.String())
	}
}

func TestFeatures_Gates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ver     string
		netQual bool
		shorts  bool
		wdutil  bool
	}{
		{"10.15", false, false, false},
		{"11.0", false, false, true},
		{"11.7", false, false, true},
		{"12.0", true, false, true},
		{"12.3", true, false, true},
		{"13.0", true, true, true},
		{"15.2", true, true, true},
		{"16.0", true, true, true},
	}
	for _, c := range cases {
		f := capability.DeriveFeatures(capability.ParseVersion(c.ver))
		if f.NetworkQuality != c.netQual || f.Shortcuts != c.shorts || f.WdutilInfo != c.wdutil {
			t.Errorf("%s → %+v, want (nq=%v sh=%v wd=%v)",
				c.ver, f, c.netQual, c.shorts, c.wdutil)
		}
	}
}

func TestDetect_UsesRunner(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sw_vers -productVersion", "15.3\n", nil)
	rep, err := capability.Detect(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Version.Major != 15 || rep.Version.Minor != 3 {
		t.Fatalf("unexpected version %+v", rep.Version)
	}
	if !rep.Features.NetworkQuality || !rep.Features.Shortcuts {
		t.Fatal("features should be enabled on 15.3")
	}
}

func TestDetect_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sw_vers -productVersion", "", errors.New("no sw_vers"))
	if _, err := capability.Detect(context.Background(), f); err == nil {
		t.Fatal("expected error propagated")
	}
}

func TestReport_Summary(t *testing.T) {
	t.Parallel()
	rep := capability.Report{
		Version: capability.ParseVersion("15.2"),
		Features: capability.Features{
			NetworkQuality: true, Shortcuts: true, WdutilInfo: true,
		},
	}
	got := rep.Summary()
	if !strings.Contains(got, "15.2") {
		t.Errorf("summary missing version: %q", got)
	}
	if !strings.Contains(got, "3/3") {
		t.Errorf("summary missing counter: %q", got)
	}
}

func TestReport_Summary_Partial(t *testing.T) {
	t.Parallel()
	rep := capability.Report{
		Version: capability.ParseVersion("11.0"),
		Features: capability.Features{
			NetworkQuality: false, Shortcuts: false, WdutilInfo: true,
		},
	}
	if !strings.Contains(rep.Summary(), "1/3") {
		t.Errorf("expected 1/3, got: %q", rep.Summary())
	}
}
