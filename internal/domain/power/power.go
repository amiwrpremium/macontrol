// Package power exposes lock / sleep / restart / shutdown /
// logout + keep-awake controls.
//
// Where macOS offers a non-sudo path, the package prefers it
// (AppleScript via osascript for the destructive actions; pmset
// for the lock + sleep paths) so the bot works without the
// narrow sudoers entry for everyday operations. Restart /
// Shutdown / Logout still require an interactive user session
// because they go through System Events.
//
// Public surface:
//
//   - [Service.Lock] — display sleep (lock if "require password
//     after sleep" is enabled).
//   - [Service.Sleep] — system sleep.
//   - [Service.Restart] / [Service.Shutdown] / [Service.Logout]
//     — destructive session actions; the Telegram UX gates each
//     behind a Confirm/Cancel dialog.
//   - [Service.KeepAwake] / [Service.CancelKeepAwake] — wrap
//     the `caffeinate` CLI to inhibit display sleep for a
//     duration.
package power

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service is the macOS power / session control surface. One
// instance per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Power.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     [Service.KeepAwake] spawns a detached background process
//     (caffeinate) that the receiver does NOT track — see the
//     smells list.
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

// Lock puts the display to sleep via `pmset displaysleepnow`.
// Whether this also locks the session depends on the user's
// "Require password after sleep" setting in System Settings →
// Privacy & Security.
//
// Behavior:
//   - Works on every macOS the bot supports (10.7 → current).
//   - Requires no sudo grant and no Accessibility permission —
//     pmset's displaysleepnow is one of the few power
//     subcommands that doesn't.
//   - Returns the runner error verbatim on pmset failure (rare).
//
// Note: the legacy `CGSession -suspend` lock-screen helper is
// no longer shipped on modern macOS; pmset displaysleepnow is
// the closest semantic preserved across releases.
func (s *Service) Lock(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pmset", "displaysleepnow")
	return err
}

// Sleep puts the entire Mac to sleep via `pmset sleepnow`.
// Differs from [Service.Lock] in that the CPU and most
// peripherals also enter low-power state, not just the display.
//
// Behavior:
//   - Returns the runner error verbatim on pmset failure.
//   - On wake, the daemon resumes whichever active flow / state
//     it had — the long-poll connection to Telegram re-establishes
//     after the network comes back up.
func (s *Service) Sleep(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pmset", "sleepnow")
	return err
}

// Restart reboots the Mac via
// `osascript -e 'tell application "System Events" to restart'`.
//
// Behavior:
//   - Requires no sudo grant — System Events handles the
//     privilege escalation through the user's authenticated
//     session.
//   - Requires that a user is currently logged in (the daemon's
//     own launchd context counts on macOS 11+).
//   - Returns the runner error verbatim on osascript failure
//     (typically "Application can't be found" if System Events
//     is somehow not running).
//
// Telegram UX: this is gated behind a Confirm/Cancel dialog by
// the Power dashboard — there's no recovery path.
func (s *Service) Restart(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to restart`)
	return err
}

// Shutdown powers off the Mac via
// `osascript -e 'tell application "System Events" to shut down'`.
// Same behavior contract and gating as [Service.Restart] —
// destructive, no recovery, requires a logged-in session.
func (s *Service) Shutdown(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to shut down`)
	return err
}

// Logout ends the current user session via
// `osascript -e 'tell application "System Events" to log out'`.
// Same behavior contract and gating as [Service.Restart].
//
// Note: dirty applications (unsaved documents, open dialogs)
// can ABORT the logout via their "Cancel" prompt; macOS will
// silently skip the logout in that case and the daemon receives
// a successful exit anyway. This is a System Events limitation.
func (s *Service) Logout(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to log out`)
	return err
}

// KeepAwake starts `caffeinate -d -t <seconds>` as a detached
// background process. Returns immediately after spawning — the
// caffeinate process exits on its own after the duration.
//
// Behavior:
//   - Validates duration > 0; returns
//     "keep-awake duration must be positive, got %s" for
//     non-positive input.
//   - Wraps the caffeinate invocation in
//     `sh -c 'nohup caffeinate -d -t <secs> >/dev/null 2>&1 &'`
//     so the shell forks caffeinate and returns. Necessary
//     because macontrol's [runner.Runner] otherwise blocks until
//     the child exits, which would make KeepAwake hold the call
//     for the entire requested duration.
//   - The receiver does NOT track the spawned pid. Multiple
//     KeepAwake calls layer; each runs independently until its
//     own duration expires. [Service.CancelKeepAwake] kills ALL
//     of them.
//
// Returns the wrapped sh error on failure (rare; sh and
// caffeinate are reliable).
func (s *Service) KeepAwake(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("keep-awake duration must be positive, got %s", d)
	}
	// Run detached: we don't want our context's timeout to kill caffeinate.
	// Use `sh -c ... &` so the shell forks caffeinate and returns immediately.
	_, err := s.r.Exec(ctx, "sh", "-c",
		fmt.Sprintf("nohup caffeinate -d -t %d >/dev/null 2>&1 &", int(d.Seconds())))
	return err
}

// CancelKeepAwake terminates every caffeinate process owned by
// the current user via `pkill -x caffeinate`.
//
// Behavior:
//   - Uses `-x` for exact match on the binary name, so
//     unrelated processes containing "caffeinate" in their
//     command line don't get killed.
//   - pkill exits 1 when no processes matched, which is the
//     "nothing to cancel" success case for us — [ignoreNoMatches]
//     translates that into a nil return.
//   - Returns any other runner error verbatim.
func (s *Service) CancelKeepAwake(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pkill", "-x", "caffeinate")
	// pkill exits 1 when no matches — treat as success.
	return ignoreNoMatches(err)
}

// ignoreNoMatches translates pkill's "no matches" exit-1 into a
// nil error so callers don't have to treat "nothing to cancel"
// as a failure.
//
// Behavior:
//   - nil input → returns nil.
//   - Wrapped [runner.Error] with underlying err.Error() ==
//     "exit status 1" → returns nil (the pkill no-match case).
//   - Anything else → returns the input verbatim.
//
// Limitation: matching on the exact string "exit status 1" is
// brittle — any other utility that exits 1 for an actual
// problem would also be silently swallowed. Mitigated by being
// called only from [Service.CancelKeepAwake] where we know the
// command is `pkill`.
func ignoreNoMatches(err error) error {
	if err == nil {
		return nil
	}
	var rerr *runner.Error
	if errors.As(err, &rerr) && rerr.Err != nil && rerr.Err.Error() == "exit status 1" {
		return nil
	}
	return err
}
