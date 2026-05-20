// Package runner is the macOS-subprocess boundary for the entire bot.
//
// Every domain capability (display brightness, Wi-Fi state, battery,
// notifications, …) eventually shells out to a macOS CLI. Routing
// those calls through [Runner] keeps the domain layer testable:
// production wires up [Exec], tests inject [Fake] (see fake.go) and
// pre-program command → result rules. The package owns three
// concerns: ordinary command execution, sudoers-aware "fail fast"
// execution for the narrow /etc/sudoers.d/macontrol entry, and
// stream-merging for CLIs that emit useful diagnostic text on stderr
// even when exiting zero.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner executes external commands on behalf of the domain layer.
// Every domain service holds a Runner and never calls os/exec
// directly, which is what lets the test suite swap in [Fake].
//
// Behavior:
//   - Every method must honour ctx cancellation. Long-running CLIs
//     (network probes, system_profiler dumps, shortcuts run) get a
//     hard cap from the implementation when no deadline is set on
//     ctx — see [Exec.DefaultTimeout] and [DefaultTimeout].
//   - Stdout is the success payload. Stderr is captured separately
//     by Exec and Sudo, and merged into stdout by ExecCombined.
//   - On non-zero exit or context cancellation, implementations
//     return a *[Error] carrying both streams and the wrapped exec
//     error. Callers may use errors.Is / errors.As to detect
//     exec.ExitError or context.DeadlineExceeded.
//   - Every method returns the partial stdout slice even on
//     failure; callers may inspect it (some CLIs emit useful text
//     before exiting non-zero).
//
// Concurrency:
//   - Implementations MUST be safe for concurrent calls from
//     multiple goroutines. [Exec] is stateless across calls so it
//     is trivially safe; [Fake] serialises rule lookups with a
//     mutex.
type Runner interface {
	// Exec runs name with args and returns stdout on success. On
	// non-zero exit it returns a *[Error] whose Stderr field carries
	// the captured error stream so the Telegram surface can relay a
	// useful message. On ctx-deadline expiry it returns *[Error]
	// with Err set to a synthetic "timed out after X" wrapping
	// context.DeadlineExceeded.
	Exec(ctx context.Context, name string, args ...string) ([]byte, error)

	// Sudo runs name with args under "sudo -n …" so missing
	// /etc/sudoers.d/macontrol entries fail immediately rather than
	// hanging waiting for a password prompt the daemon has no TTY
	// to satisfy. Error shape is otherwise identical to [Runner.Exec].
	// Note: the *[Error] built for a Sudo failure currently carries
	// Cmd = "sudo" and Args = ["-n", name, args…], not the
	// user-visible command — see the smells list in the package
	// review notes.
	Sudo(ctx context.Context, name string, args ...string) ([]byte, error)

	// ExecCombined runs name with args and returns stdout+stderr
	// merged into a single buffer. Use it for CLIs that put real
	// information on stderr even when exiting zero (notably the
	// `brightness` CLI under CoreDisplay denial). On failure, the
	// *[Error] has its Stdout field carrying the merged stream and
	// its Stderr field empty.
	ExecCombined(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Error is the failure shape returned by every [Runner] method.
// Designed so the Telegram surface can render an actionable single
// line without dumping the full process streams.
//
// Lifecycle:
//   - Constructed by [Exec.run] and [Exec.ExecCombined] when the
//     child process exits non-zero or the context deadline fires.
//     Never constructed by callers.
//   - Returned alongside any stdout already captured before the
//     failure — callers receive both the partial bytes and this
//     error.
//
// Field roles:
//
//   - Cmd / Args together describe what was actually executed. For
//     Exec and ExecCombined these are the user-visible command +
//     args. For Sudo, Cmd is "sudo" and Args includes the "-n"
//     prefix and the wrapped command (see the [Runner.Sudo] note).
//
//   - Stdout / Stderr are the captured bytes up to the failure.
//     Stderr is empty for [Runner.ExecCombined] (both streams merge
//     into Stdout).
//
//   - Err is the wrapped error. Either an *exec.ExitError from the
//     child, or a synthetic "timed out after X" wrapping
//     context.DeadlineExceeded. errors.Is and errors.As work
//     against either.
type Error struct {
	// Cmd is the bare command name as seen by os/exec. For Sudo
	// failures this is "sudo".
	Cmd string

	// Args is the argument slice passed to Cmd. For Sudo failures
	// this includes the "-n" flag and the wrapped command name.
	Args []string

	// Stdout is the standard-output bytes captured up to the
	// failure (or the merged stream for ExecCombined).
	Stdout []byte

	// Stderr is the standard-error bytes captured up to the
	// failure. Empty for ExecCombined since stdout and stderr
	// merge into Stdout.
	Stderr []byte

	// Err is the underlying exec error from cmd.Run, or a
	// synthetic "timed out after X" error wrapping
	// context.DeadlineExceeded when the timeout fires.
	Err error
}

// Error formats the failure as
// "cmd args: underlying-err: <stderr-or-stdout snippet>" so the
// Telegram surface can relay one actionable line without dumping
// the full streams.
//
// Behavior:
//   - Joins Cmd and Args with single spaces.
//   - Picks the first non-empty of Stderr, then Stdout, for the
//     trailing snippet. Whitespace is trimmed.
//   - When both streams are empty the snippet is omitted entirely
//     and the format becomes "cmd args: underlying-err".
func (e *Error) Error() string {
	cmd := e.Cmd
	if len(e.Args) > 0 {
		cmd += " " + strings.Join(e.Args, " ")
	}
	trimmed := strings.TrimSpace(string(e.Stderr))
	if trimmed == "" {
		trimmed = strings.TrimSpace(string(e.Stdout))
	}
	if trimmed == "" {
		return fmt.Sprintf("%s: %v", cmd, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", cmd, e.Err, trimmed)
}

// Unwrap returns the wrapped exec or timeout error so callers can
// use errors.Is / errors.As to detect *exec.ExitError or
// context.DeadlineExceeded.
func (e *Error) Unwrap() error { return e.Err }

// DefaultTimeout is the per-call cap applied when the caller's
// context has no deadline. Keeps the bot responsive even if a
// macOS CLI hangs indefinitely. Per-call overrides live on
// [Exec.DefaultTimeout]; the wifi speedtest is the only known
// caller that needs a longer cap (see internal/domain/wifi).
const DefaultTimeout = 15 * time.Second

// Exec is the production [Runner] backed by os/exec.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps, and shared by every domain service for the
//     lifetime of the process.
//
// Concurrency:
//   - Stateless across calls — each invocation spins up its own
//     *exec.Cmd plus stdout/stderr buffers. Safe to call from any
//     number of goroutines.
//
// Field roles:
//   - DefaultTimeout overrides the package-level [DefaultTimeout]
//     for callers that need a non-15s cap (e.g. networkQuality
//     speed test gets 60s). Zero falls through to the package
//     default.
type Exec struct {
	// DefaultTimeout overrides the package-level cap. Zero means
	// fall back to [DefaultTimeout].
	DefaultTimeout time.Duration
}

// New returns an [Exec] with the package-default timeout. The
// returned value satisfies [Runner].
func New() *Exec { return &Exec{} }

// Exec satisfies [Runner.Exec]. See the interface doc for the
// full behavior contract; this implementation captures stdout and
// stderr into separate buffers via the shared [Exec.run] helper.
func (e *Exec) Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	return e.run(ctx, name, args)
}

// Sudo satisfies [Runner.Sudo]. Prepends "-n" so missing sudoers
// entries surface as a fast non-zero exit ("sudo: a password is
// required") rather than blocking forever on a TTY-less password
// prompt.
//
// Behavior:
//   - Builds args = ["-n", name, args…] and dispatches through
//     [Exec.run] with Cmd = "sudo".
//   - The resulting *[Error] therefore carries Cmd = "sudo" and
//     Args = ["-n", <wrapped-name>, …] — see the smells list.
func (e *Exec) Sudo(ctx context.Context, name string, args ...string) ([]byte, error) {
	full := make([]string, 0, len(args)+2)
	full = append(full, "-n", name)
	full = append(full, args...)
	return e.run(ctx, "sudo", full)
}

// ExecCombined satisfies [Runner.ExecCombined]. Wires the child
// process's stdout AND stderr to a single bytes.Buffer so callers
// see the merged stream as the success payload.
//
// Behavior:
//   - Identical timeout handling to [Exec.run]; uses
//     [Exec.withTimeout] to cap the call when ctx has no deadline.
//   - On context-deadline failure, builds *[Error] with the
//     merged stream as Stdout, no Stderr, and Err = "timed out
//     after X" wrapping context.DeadlineExceeded.
//   - On other failures (non-zero exit), same shape but Err is
//     the underlying *exec.ExitError.
func (e *Exec) ExecCombined(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := e.withTimeout(ctx)
	defer cancel()

	var combined bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return combined.Bytes(), &Error{
				Cmd: name, Args: args,
				Stdout: combined.Bytes(),
				Err:    fmt.Errorf("timed out after %s", e.timeout()),
			}
		}
		return combined.Bytes(), &Error{
			Cmd: name, Args: args,
			Stdout: combined.Bytes(), Err: err,
		}
	}
	return combined.Bytes(), nil
}

// run is the shared implementation behind [Exec.Exec] and
// [Exec.Sudo]. Captures stdout and stderr into separate buffers,
// applies the timeout via [Exec.withTimeout], and translates
// non-zero exits or deadline-exceeded into *[Error] preserving the
// captured streams.
func (e *Exec) run(ctx context.Context, name string, args []string) ([]byte, error) {
	ctx, cancel := e.withTimeout(ctx)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.Bytes(), &Error{
				Cmd: name, Args: args,
				Stdout: stdout.Bytes(), Stderr: stderr.Bytes(),
				Err: fmt.Errorf("timed out after %s", e.timeout()),
			}
		}
		return stdout.Bytes(), &Error{
			Cmd: name, Args: args,
			Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Err: err,
		}
	}
	return stdout.Bytes(), nil
}

// timeout returns the per-call cap: [Exec.DefaultTimeout] if
// non-zero, otherwise the package [DefaultTimeout].
func (e *Exec) timeout() time.Duration {
	if e.DefaultTimeout > 0 {
		return e.DefaultTimeout
	}
	return DefaultTimeout
}

// withTimeout wraps ctx with the per-call cap from [Exec.timeout]
// only when ctx has no existing deadline. Callers that pass a
// deadline (e.g. a request scoped to a Telegram update) keep their
// original budget; callers that don't (e.g. internal background
// pings) get the safety cap. The returned cancel is a no-op in the
// "deadline already set" branch.
func (e *Exec) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, e.timeout())
}
