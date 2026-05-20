package callbacks_test

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// FuzzDecode exercises [callbacks.Decode] against arbitrary input
// to catch panics and unbounded resource use. Decode is the only
// attacker-reachable parser before the whitelist gate, so its
// resilience matters even though the realistic input vocabulary
// is small.
//
// The contract under test:
//
//   - Decode must never panic, regardless of input.
//   - Returning a non-nil error is acceptable; that's the
//     documented signal for malformed input.
//   - Returning a Data without error is acceptable; correctness
//     of the parsed fields is covered by [TestEncodeDecode_Roundtrip]
//     and the targeted reject tests in data_test.go.
//
// We deliberately do NOT assert on the returned values — fuzzing
// is for resilience, not correctness. Unit tests own correctness.
func FuzzDecode(f *testing.F) {
	// Seed corpus. Mix of:
	//   - realistic shapes from production keyboards;
	//   - edge cases the hand-written tests already cover;
	//   - boundary lengths around MaxCallbackDataBytes;
	//   - non-ASCII / control characters.
	seeds := []string{
		// Production-shaped callbacks.
		"snd:up:5",
		"snd:open",
		"snd:set",
		"dsp:down:10",
		"pwr:restart",
		"pwr:restart:ok",
		"wif:dns:cf",
		"wif:open",
		"bat:health",
		"sys:proc:1234",
		"sys:kill9:1234:ok",
		"tls:tz-set:abc123",
		"tls:disk:xyz789",
		"med:shot:silent",
		"nav:home",

		// Empty / minimal.
		"",
		":",
		"::",
		":::",
		":a",
		"a:",
		"a:b",
		"a:b:",
		":a:b",

		// Boundary lengths.
		strings.Repeat("a", callbacks.MaxCallbackDataBytes),
		strings.Repeat("a", callbacks.MaxCallbackDataBytes+1),
		strings.Repeat(":", callbacks.MaxCallbackDataBytes),
		strings.Repeat(":", callbacks.MaxCallbackDataBytes+1),
		"snd:up:" + strings.Repeat("x", callbacks.MaxCallbackDataBytes-7),

		// Control / NUL characters.
		"snd:up:\x00",
		"\x00:b:c",
		"snd:\n:5",
		"snd:up:\t",
		"snd:up:\xff",

		// Unicode (multi-byte runes).
		"snd:up:café",
		"snd:up:🎵",
		"💡:open",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = callbacks.Decode(raw)
	})
}
