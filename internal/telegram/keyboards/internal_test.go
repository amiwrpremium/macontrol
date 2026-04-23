package keyboards

import "testing"

// atLeastOne and plural are unexported helpers used by the tls.go
// header builders. Cover their thresholds directly.

func TestAtLeastOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want int
	}{
		{-5, 1},
		{-1, 1},
		{0, 1},
		{1, 1},
		{2, 2},
		{5, 5},
		{1000, 1000},
	}
	for _, c := range cases {
		if got := atLeastOne(c.in); got != c.want {
			t.Errorf("atLeastOne(%d) = %d; want %d", c.in, got, c.want)
		}
	}
}

func TestPlural(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n      int
		suffix string
		want   string
	}{
		{0, "s", "s"},
		{1, "s", ""},
		{2, "s", "s"},
		{0, "es", "es"},
		{1, "es", ""},
		{5, "es", "es"},
		{-1, "s", "s"},   // anything that isn't 1 → suffix
		{-1, "es", "es"}, // negative count keeps the suffix
	}
	for _, c := range cases {
		if got := plural(c.n, c.suffix); got != c.want {
			t.Errorf("plural(%d, %q) = %q; want %q", c.n, c.suffix, got, c.want)
		}
	}
}

func TestFilterIDArg(t *testing.T) {
	t.Parallel()
	if got := filterIDArg(""); got != "-" {
		t.Errorf("filterIDArg(\"\") = %q; want %q", got, "-")
	}
	if got := filterIDArg("xyz"); got != "xyz" {
		t.Errorf("filterIDArg(\"xyz\") = %q; want %q", got, "xyz")
	}
}
