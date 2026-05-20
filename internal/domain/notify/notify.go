// Package notify sends desktop notifications and text-to-speech.
//
// Two transports for desktop notifications:
//
//   - terminal-notifier (preferred) — a brew formula that exposes
//     a richer notification UX (group, sound, click-action) and
//     is more reliable across macOS releases.
//   - osascript display notification (fallback) — built into
//     macOS so the daemon always has SOMETHING to call, but the
//     resulting notification cannot be made actionable and may
//     be silently rate-limited by NotificationCenter.
//
// [Service.Notify] picks the transport at call time via
// `exec.LookPath("terminal-notifier")` and reports back which one
// it used so the handler can surface that to the user.
//
// Text-to-speech goes through the built-in `say` CLI directly via
// [Service.Say]; there's no fallback (say is part of macOS).
package notify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Opts tunes a desktop notification. Same shape regardless of
// which transport ([Service.viaTerminalNotifier] or
// [Service.viaOsascript]) services it; each transport maps the
// fields onto its own flags.
//
// Field roles:
//   - Title is the bold first line; empty means "no title."
//   - Body is the message text. The notification flow always
//     supplies one (it's the part the user types into the
//     "title | body" prompt) but [Service.Notify] also accepts
//     "title only" as long as Title is non-empty.
//   - Sound is the macOS sound name to play (e.g. "default",
//     "Glass", "Ping"). Empty means silent. The valid set is the
//     contents of /System/Library/Sounds.
type Opts struct {
	// Title is the bold first line of the notification.
	Title string

	// Body is the notification message text.
	Body string

	// Sound is the macOS sound name to play, e.g. "default" /
	// "Glass" / "Ping". Empty means silent. Names match files
	// under /System/Library/Sounds (without the .aiff suffix).
	Sound string
}

// Service is the notification + TTS control surface. One
// instance per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Notify.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
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

// Notify sends a desktop notification.
//
// Behavior:
//   - Validates that at least one of Title or Body is set;
//     returns "notify: title or body required" otherwise.
//   - Prefers terminal-notifier when [Service.hasTerminalNotifier]
//     reports it on $PATH.
//   - Falls back to `osascript display notification` when
//     terminal-notifier is missing.
//
// Returns the transport name ("terminal-notifier" or
// "osascript") alongside the underlying transport error so the
// handler can surface "Notified via terminal-notifier" / "Notified
// via osascript" — useful when debugging which path actually ran.
func (s *Service) Notify(ctx context.Context, o Opts) (transport string, err error) {
	if o.Title == "" && o.Body == "" {
		return "", errors.New("notify: title or body required")
	}
	if s.hasTerminalNotifier() {
		return "terminal-notifier", s.viaTerminalNotifier(ctx, o)
	}
	return "osascript", s.viaOsascript(ctx, o)
}

// Say speaks text through the default macOS text-to-speech
// voice via the built-in `say` CLI.
//
// Behavior:
//   - Rejects empty / whitespace-only text with "say: text is
//     empty".
//   - Runs `say <text>` with text as a single positional arg
//     (no shell quoting concerns).
//   - Returns the runner error verbatim on `say` failure (rare).
//
// Note: `say` blocks until speech completes; long inputs hold
// the call for the duration of the spoken text and may exceed
// the runner's default 15-s timeout. See the smells list on
// sound.go for the same caveat (this Say is a peer
// implementation; the sound package has its own).
func (s *Service) Say(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("say: text is empty")
	}
	_, err := s.r.Exec(ctx, "say", text)
	return err
}

// hasTerminalNotifier reports whether the `terminal-notifier`
// brew formula is on $PATH. Used by [Service.Notify] to pick
// the transport.
//
// Implementation note: uses [exec.LookPath] directly rather than
// going through the runner because LookPath is a pure $PATH
// query (no subprocess) and the runner doesn't expose it. This
// means the check IS NOT mockable via [runner.Fake] — tests that
// want to force one transport or the other must arrange the
// host's $PATH instead. See the smells list.
func (s *Service) hasTerminalNotifier() bool {
	_, err := exec.LookPath("terminal-notifier")
	return err == nil
}

// viaTerminalNotifier sends one notification via the
// `terminal-notifier` brew formula. Stamps `-group macontrol`
// so multiple notifications from the daemon collapse into a
// single Notification Center entry rather than stacking.
//
// Behavior:
//   - Always passes -group macontrol.
//   - Passes -title only when Opts.Title is non-empty.
//   - Always passes -message (terminal-notifier requires it,
//     but [Service.Notify]'s validation already ensures at
//     least one of Title/Body is set so passing an empty body
//     after a non-empty Title is acceptable).
//   - Passes -sound only when Opts.Sound is non-empty.
func (s *Service) viaTerminalNotifier(ctx context.Context, o Opts) error {
	args := []string{"-group", "macontrol"}
	if o.Title != "" {
		args = append(args, "-title", o.Title)
	}
	args = append(args, "-message", o.Body)
	if o.Sound != "" {
		args = append(args, "-sound", o.Sound)
	}
	_, err := s.r.Exec(ctx, "terminal-notifier", args...)
	return err
}

// viaOsascript sends one notification via
// `osascript -e 'display notification "<body>" with title
// "<title>" sound name "<sound>"'`.
//
// Behavior:
//   - Builds the AppleScript fragment with %q-quoted values so
//     embedded quotes / backslashes round-trip safely (Go's %q
//     uses Go-string escaping which AppleScript's lexer
//     accepts for ASCII).
//   - Title and Sound clauses are appended only when
//     respective Opts fields are non-empty.
//   - Returns the runner error verbatim on osascript failure.
//
// Caveat: the AppleScript transport doesn't support -group or
// click actions; multiple notifications stack in
// NotificationCenter rather than collapsing.
func (s *Service) viaOsascript(ctx context.Context, o Opts) error {
	var b strings.Builder
	fmt.Fprintf(&b, `display notification %q`, o.Body)
	if o.Title != "" {
		fmt.Fprintf(&b, ` with title %q`, o.Title)
	}
	if o.Sound != "" {
		fmt.Fprintf(&b, ` sound name %q`, o.Sound)
	}
	_, err := s.r.Exec(ctx, "osascript", "-e", b.String())
	return err
}
