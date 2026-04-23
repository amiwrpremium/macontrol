package handlers

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// ---------------- labelFor ----------------

func TestLabelFor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		action, want string
	}{
		{"restart", "restart"},
		{"shutdown", "shutdown"},
		{"logout", "logout"},
		{"unknown", "unknown"}, // default branch returns the input
		{"", ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.action, func(t *testing.T) {
			t.Parallel()
			if got := labelFor(c.action); got != c.want {
				t.Errorf("labelFor(%q) = %q; want %q", c.action, got, c.want)
			}
		})
	}
}

// ---------------- pressureLabel ----------------

func TestPressureLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		freePct int
		want    string
	}{
		{100, "Normal"},
		{50, "Normal"},
		{30, "Normal"}, // boundary
		{29, "Warning"},
		{15, "Warning"},
		{10, "Warning"}, // boundary
		{9, "Critical"},
		{5, "Critical"},
		{0, "Critical"},
		{-1, "Critical"}, // negative free is impossible but the switch defaults to Critical
	}
	for _, c := range cases {
		if got := pressureLabel(c.freePct); got != c.want {
			t.Errorf("pressureLabel(%d) = %q; want %q", c.freePct, got, c.want)
		}
	}
}

// ---------------- pidArg ----------------

func TestPidArg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args    []string
		wantPid int
		wantOK  bool
	}{
		{nil, 0, false},
		{[]string{}, 0, false},
		{[]string{"123"}, 123, true},
		{[]string{"1"}, 1, true},
		{[]string{"99999"}, 99999, true},
		{[]string{"0"}, 0, false},     // pidArg requires positive
		{[]string{"-5"}, 0, false},    // negative rejected
		{[]string{"abc"}, 0, false},   // non-numeric rejected
		{[]string{""}, 0, false},      // empty rejected
		{[]string{"  42"}, 0, false},  // leading whitespace rejected by Atoi
		{[]string{"1", "2"}, 1, true}, // extra args ignored
	}
	for _, c := range cases {
		got, ok := pidArg(callbacks.Data{Args: c.args})
		if got != c.wantPid || ok != c.wantOK {
			t.Errorf("pidArg(%v) = (%d, %v); want (%d, %v)", c.args, got, ok, c.wantPid, c.wantOK)
		}
	}
}

// ---------------- percentOf ----------------

func TestPercentOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		num, denom uint64
		want       int
	}{
		{0, 0, 0},  // denom==0 → 0
		{50, 0, 0}, // denom==0 short-circuits
		{50, 100, 50},
		{0, 100, 0},
		{100, 100, 100},
		{200, 100, 100}, // clamped to 100
		{1, 3, 33},
		{2, 3, 66},
	}
	for _, c := range cases {
		if got := percentOf(c.num, c.denom); got != c.want {
			t.Errorf("percentOf(%d, %d) = %d; want %d", c.num, c.denom, got, c.want)
		}
	}
}

// ---------------- truncate ----------------

func TestTruncate(t *testing.T) {
	t.Parallel()
	if got := truncate("short", 100); got != "short" {
		t.Errorf("short input untouched; got %q", got)
	}
	long := strings.Repeat("x", 200)
	got := truncate(long, 50)
	if !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("expected (truncated) suffix; got %q", got)
	}
	if !strings.HasPrefix(got, strings.Repeat("x", 50)) {
		t.Errorf("expected first 50 x's; got %q", got)
	}
	// Boundary: exactly n bytes — should pass through.
	exact := strings.Repeat("y", 10)
	if got := truncate(exact, 10); got != exact {
		t.Errorf("exact-length input should pass through; got %q", got)
	}
}

// ---------------- nonEmpty ----------------

func TestNonEmpty(t *testing.T) {
	t.Parallel()
	if got := nonEmpty(""); got != "?" {
		t.Errorf("empty → %q; want %q", got, "?")
	}
	if got := nonEmpty("hello"); got != "hello" {
		t.Errorf("non-empty passthrough failed; got %q", got)
	}
	if got := nonEmpty(" "); got != " " {
		t.Errorf("whitespace is non-empty; got %q", got)
	}
}

// ---------------- leafOfPath ----------------

func TestLeafOfPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"/Applications/Safari.app/Contents/MacOS/Safari", "Safari"},
		{"WindowServer", "WindowServer"},
		{"", ""},
		{"/usr/bin/sudo", "sudo"},
		{"/", ""},
	}
	for _, c := range cases {
		if got := leafOfPath(c.in); got != c.want {
			t.Errorf("leafOfPath(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// ---------------- fmtBytes / humanBytes ----------------

func TestFmtBytes(t *testing.T) {
	t.Parallel()
	if got := fmtBytes(0); got != "?" {
		t.Errorf("zero → %q; want '?'", got)
	}
	if got := fmtBytes(8 * 1024 * 1024 * 1024); got != "8 GiB" {
		t.Errorf("8 GiB exact → %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()
	if got := humanBytes(0); got != "?" {
		t.Errorf("zero → %q", got)
	}
	if got := humanBytes(2 * 1024 * 1024); got != "2 MiB" {
		t.Errorf("2 MiB → %q", got)
	}
	if got := humanBytes(2 * 1024 * 1024 * 1024); got != "2.0 GiB" {
		t.Errorf("2 GiB → %q", got)
	}
}

// ---------------- isConfirm ----------------

func TestIsConfirm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"ok"}, true},
		{[]string{}, false},
		{nil, false},
		{[]string{"cancel"}, false},
		{[]string{"OK"}, false}, // case-sensitive
		{[]string{"ok", "extra"}, true},
	}
	for _, c := range cases {
		if got := isConfirm(c.args); got != c.want {
			t.Errorf("isConfirm(%v) = %v; want %v", c.args, got, c.want)
		}
	}
}

// ---------------- parseShortcutPageArgs ----------------

func TestParseShortcutPageArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args         []string
		wantPage     int
		wantFilterID string
	}{
		{nil, 0, ""},
		{[]string{}, 0, ""},
		{[]string{"3"}, 3, ""},
		{[]string{"3", "abc"}, 3, "abc"},
		{[]string{"3", "-"}, 3, ""}, // sentinel
		{[]string{"-5"}, 0, ""},     // negative clamps to 0
		{[]string{"bad"}, 0, ""},    // non-numeric → 0
	}
	for _, c := range cases {
		gotPage, gotID := parseShortcutPageArgs(callbacks.Data{Args: c.args})
		if gotPage != c.wantPage || gotID != c.wantFilterID {
			t.Errorf("parseShortcutPageArgs(%v) = (%d, %q); want (%d, %q)",
				c.args, gotPage, gotID, c.wantPage, c.wantFilterID)
		}
	}
}

func TestParseShortcutPageArgsAt(t *testing.T) {
	t.Parallel()
	// Offset 1 — first arg is consumed by the caller (e.g. sc-run shortID).
	args := []string{"shortID", "2", "filt"}
	page, fid := parseShortcutPageArgsAt(callbacks.Data{Args: args}, 1)
	if page != 2 || fid != "filt" {
		t.Errorf("offset 1: page=%d fid=%q", page, fid)
	}
	// Sentinel at offset 1.
	page, fid = parseShortcutPageArgsAt(callbacks.Data{Args: []string{"x", "0", "-"}}, 1)
	if page != 0 || fid != "" {
		t.Errorf("sentinel: page=%d fid=%q", page, fid)
	}
}

// ---------------- groupTimezones ----------------

func TestGroupTimezones(t *testing.T) {
	t.Parallel()
	all := []string{
		"America/New_York", "Asia/Tehran", "Europe/Berlin",
		"Europe/Istanbul", "GMT", "UTC", "America/Los_Angeles",
	}
	regions, topLevels := groupTimezones(all)
	// 3 regions: America, Asia, Europe.
	if len(regions) != 3 {
		t.Fatalf("expected 3 regions, got %d", len(regions))
	}
	// Sorted alphabetically.
	wantOrder := []string{"America", "Asia", "Europe"}
	for i, r := range regions {
		if r.Slug != wantOrder[i] {
			t.Errorf("region[%d].Slug = %q; want %q", i, r.Slug, wantOrder[i])
		}
	}
	// America has 2 entries; Europe has 2.
	for _, r := range regions {
		switch r.Slug {
		case "America":
			if len(r.Tzs) != 2 {
				t.Errorf("America has %d entries, want 2", len(r.Tzs))
			}
		case "Europe":
			if len(r.Tzs) != 2 {
				t.Errorf("Europe has %d entries, want 2", len(r.Tzs))
			}
		}
	}
	// GMT and UTC are top-level (no '/').
	if len(topLevels) != 2 {
		t.Fatalf("expected 2 top-level, got %v", topLevels)
	}
}

func TestGroupTimezones_Empty(t *testing.T) {
	t.Parallel()
	regions, topLevels := groupTimezones(nil)
	if len(regions) != 0 || len(topLevels) != 0 {
		t.Errorf("empty input should yield empty groups; got regions=%v topLevels=%v",
			regions, topLevels)
	}
}
