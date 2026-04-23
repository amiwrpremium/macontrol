// Package runner wraps os/exec for the macOS subprocess calls that back every
// domain capability. The public Runner interface keeps the domain layer
// testable without touching any real binaries.
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

// Runner executes external commands. Implementations must honor ctx
// cancellation and return the contents of stdout on success.
type Runner interface {
	// Exec runs name with args and returns stdout. Non-zero exit codes yield
	// an *Error carrying stdout, stderr, and the underlying exec error.
	Exec(ctx context.Context, name string, args ...string) ([]byte, error)

	// Sudo prepends "sudo -n" so the call fails fast (rather than prompting)
	// if the narrow /etc/sudoers.d/macontrol entry is not installed.
	Sudo(ctx context.Context, name string, args ...string) ([]byte, error)

	// ExecCombined runs name with args and returns stdout+stderr merged
	// into one byte slice. Use it for tools that emit informational or
	// error text on stderr even when exiting 0 (notably the `brightness`
	// CLI under CoreDisplay denial). Same timeout and error semantics
	// as Exec.
	ExecCombined(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Error carries both streams plus the underlying error so callers can
// produce actionable messages in the Telegram UI.
type Error struct {
	Cmd    string
	Args   []string
	Stdout []byte
	Stderr []byte
	Err    error
}

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

func (e *Error) Unwrap() error { return e.Err }

// DefaultTimeout is applied when the caller's context has no deadline.
// Keeps the bot responsive even if a macOS CLI hangs.
const DefaultTimeout = 15 * time.Second

// Exec is the production runner.
type Exec struct {
	// DefaultTimeout overrides the package default. Zero means the package
	// default (15s).
	DefaultTimeout time.Duration
}

// New returns a Runner that shells out via os/exec.
func New() *Exec { return &Exec{} }

// Exec implements Runner.
func (e *Exec) Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	return e.run(ctx, name, args)
}

// Sudo implements Runner. It prepends "-n" so missing sudoers entries fail
// immediately instead of waiting for a TTY that the daemon will never have.
func (e *Exec) Sudo(ctx context.Context, name string, args ...string) ([]byte, error) {
	full := make([]string, 0, len(args)+2)
	full = append(full, "-n", name)
	full = append(full, args...)
	return e.run(ctx, "sudo", full)
}

// ExecCombined implements Runner — same flow as Exec but writes both
// stdout and stderr into a single buffer.
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

func (e *Exec) timeout() time.Duration {
	if e.DefaultTimeout > 0 {
		return e.DefaultTimeout
	}
	return DefaultTimeout
}

func (e *Exec) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, e.timeout())
}
