package apps

import (
	"strings"
	"testing"
)

// FuzzParseAppsListing exercises [parseAppsListing] against
// arbitrary input to catch panics and unbounded resource use.
//
// The parser consumes osascript stdout, which is in principle
// constrained (we wrote the AppleScript), but the listing flows
// through a subprocess boundary and reflects user-controlled app
// names. A misbehaving app or a localized rename could in
// principle inject control characters, embedded delimiters, or
// huge runs — the parser must stay panic-free across the lot.
//
// The contract under test:
//
//   - parseAppsListing must never panic, regardless of input.
//   - Returning an empty slice is acceptable; that's the
//     documented signal for malformed-only input.
//   - Returning a slice of [App] without error is acceptable;
//     correctness of the parsed fields is covered by the table
//     tests in apps_test.go.
//
// We deliberately do NOT assert on the returned values — fuzzing
// is for resilience, not correctness. Unit tests own correctness.
func FuzzParseAppsListing(f *testing.F) {
	// Seed corpus. Mix of:
	//   - realistic shapes from the live osascript output;
	//   - edge cases the table tests already cover;
	//   - boundary lengths and adversarial inputs.
	seeds := []string{
		// Realistic single-line + multi-line listings.
		"Safari|1234|false\n",
		"Safari|1234|false\nMail|2345|true\n",
		"Visual Studio Code|9876|false\nFinder|2|false\n",

		// Empty / minimal.
		"",
		"\n",
		"\n\n\n",
		"|",
		"||",
		"|||",

		// Field-count edge cases.
		"NoPipes\n",
		"One|Two\n",
		"One|Two|Three|Four\n",

		// Numeric-pid edge cases.
		"App|notanint|false\n",
		"App|-1|false\n",
		"App|0|false\n",
		"App|99999999999999999999|false\n",
		"App||false\n",

		// Hidden-flag edge cases.
		"App|1|maybe\n",
		"App|1|TRUE\n",
		"App|1|False\n",
		"App|1|\n",

		// Whitespace handling.
		"  App  |  1  |  false  \n",
		"\tApp\t|\t1\t|\tfalse\t\n",

		// Trailing / leading newlines.
		"App|1|false",
		"\nApp|1|false\n",

		// Unicode + control characters in names.
		"Café Notes|1|false\n",
		"🎵 Music|1|false\n",
		"App\x00Name|1|false\n",
		"App\tWith\tTabs|1|false\n",

		// Embedded delimiters.
		"Foo|Bar|Baz|1|false\n",
		"App\nName|1|false\n",

		// Boundary sizes.
		strings.Repeat("A", 1024) + "|1|false\n",
		strings.Repeat("App|1|false\n", 100),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		_ = parseAppsListing(raw)
	})
}
