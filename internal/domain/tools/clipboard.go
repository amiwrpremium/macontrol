package tools

import (
	"context"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service is the catch-all "Tools" domain surface, hosting every
// macOS capability that doesn't have a dedicated package: the
// clipboard, the IANA timezone picker, the disks list + per-disk
// drill-down (in disks.go), and the Shortcuts.app runner (in
// shortcuts.go).
//
// One Service per process backs the Tools dashboard category; the
// keyboard layer's `tls:` namespace handler dispatches every Tools
// callback into one of these methods.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Tools.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations
//     (the [runner.Runner] is itself concurrent-safe).
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// ClipboardRead returns the current clipboard text via `pbpaste`.
//
// Behavior:
//   - Returns the verbatim stdout (NOT trimmed) so multi-line
//     clipboards round-trip exactly.
//   - Returns the runner error verbatim on `pbpaste` failure
//     (rare; pbpaste is reliable on macOS).
//
// Note: pbpaste returns binary clipboard contents (e.g. an image
// copied to the clipboard) as raw bytes; the Telegram surface
// truncates / rejects the result downstream. This package does
// not detect or convert non-text clipboards.
func (s *Service) ClipboardRead(ctx context.Context) (string, error) {
	out, err := s.r.Exec(ctx, "pbpaste")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ClipboardWrite replaces the clipboard contents with text.
//
// Behavior:
//   - Routes through `osascript -e 'set the clipboard to "<text>"'`
//     instead of piping to pbcopy. Reason: the [runner.Runner]
//     interface does not expose stdin to the child process, and
//     pbcopy reads its payload from stdin. AppleScript accepts
//     the value as an inline string argument so it works inside
//     our existing runner.
//   - Escapes backslashes and double-quotes via
//     [escapeForAppleScript] so arbitrary user input keeps the
//     AppleScript literal well-formed.
//   - Returns the runner error verbatim on osascript failure.
//
// Limitation: AppleScript string literals don't accept newlines
// directly — multi-line input gets pasted as-typed by the user
// (Telegram delivers the text with embedded \n) which AppleScript
// then interprets literally. The escape function only handles
// quote/backslash; newlines pass through.
func (s *Service) ClipboardWrite(ctx context.Context, text string) error {
	// Use AppleScript to avoid piping to pbcopy through our runner (which
	// doesn't expose stdin). Escape double-quotes and backslashes so the
	// AppleScript string literal remains well-formed for arbitrary input.
	escaped := escapeForAppleScript(text)
	_, err := s.r.Exec(ctx, "osascript", "-e",
		`set the clipboard to "`+escaped+`"`)
	return err
}

// escapeForAppleScript escapes backslashes and double-quotes in s
// so the result can be embedded inside an AppleScript double-quoted
// string literal without breaking the parse.
//
// Behavior:
//   - Walks bytes (NOT runes) — AppleScript treats the literal
//     as bytes and the only meaningful characters here are
//     ASCII '\' and '"'.
//   - Other characters (including newlines, tabs, multi-byte
//     UTF-8 sequences) pass through unchanged.
//   - Allocates the result with `len(s)` capacity; over-allocates
//     by at most the count of escape characters.
//
// Returns the escaped string. Used by [Service.ClipboardWrite].
func escapeForAppleScript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
