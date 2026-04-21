// Package power exposes lock/sleep/restart/shutdown/logout + keep-awake
// control. All operations prefer non-sudo AppleScript paths where possible
// so the bot works without sudoers entries for simple cases.
package power

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service controls macOS power/session actions.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Lock puts the display to sleep. Works on macOS 10.7 → current and
// requires no sudo or Accessibility grant. Whether this also locks
// the session depends on the user's "Require password after sleep"
// setting in System Settings → Privacy & Security; that's the same
// semantic the legacy `CGSession -suspend` relied on, and the
// `User.menu` CGSession helper no longer ships on modern macOS.
func (s *Service) Lock(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pmset", "displaysleepnow")
	return err
}

// Sleep puts the Mac to sleep.
func (s *Service) Sleep(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pmset", "sleepnow")
	return err
}

// Restart reboots via AppleScript (no sudo needed; user must be logged in).
func (s *Service) Restart(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to restart`)
	return err
}

// Shutdown powers off via AppleScript.
func (s *Service) Shutdown(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to shut down`)
	return err
}

// Logout ends the user session.
func (s *Service) Logout(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "osascript", "-e", `tell application "System Events" to log out`)
	return err
}

// KeepAwake starts `caffeinate -d -t <seconds>` asynchronously — it returns
// after kicking off the command. The caffeinate process exits on its own
// after the duration; we do not track its pid (multiple calls simply layer).
//
// Passing a non-positive duration returns an error without starting anything.
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

// CancelKeepAwake kills any running caffeinate processes owned by the user.
func (s *Service) CancelKeepAwake(ctx context.Context) error {
	_, err := s.r.Exec(ctx, "pkill", "-x", "caffeinate")
	// pkill exits 1 when no matches — treat as success.
	return ignoreNoMatches(err)
}

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
