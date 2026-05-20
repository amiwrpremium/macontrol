// Package keychain wraps the macOS `security` CLI to read, write,
// and delete generic-password Keychain entries owned by macontrol.
//
// The bot stores two secrets in the user's login keychain, each
// under a distinct service identifier so they can be queried and
// removed independently:
//
//	com.amiwrpremium.macontrol           — bot token
//	com.amiwrpremium.macontrol.whitelist — comma-separated user IDs
//
// All shell-outs route through [runner.Runner] so unit tests can
// drive the package with [runner.Fake] instead of the real
// security binary. The package recognises two macOS-specific error
// modes ([ErrNotFound], [ErrLocked]) and surfaces them as typed
// sentinels; everything else is wrapped with the failed
// service/account context.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service identifiers used by macontrol. Exposed so callers
// (tests, the setup wizard, the whitelist CLI) don't need to
// hardcode the bundle-style strings. Despite the `#nosec G101`
// hint these are NOT credentials — they're just stable lookup
// keys for the security(1) CLI's `-s` argument.
const (
	// ServiceToken is the Keychain service identifier under which
	// the Telegram bot token is stored.
	ServiceToken = "com.amiwrpremium.macontrol" // #nosec G101 -- not a credential

	// ServiceWhitelist is the Keychain service identifier under
	// which the comma-separated list of allowed Telegram user
	// IDs is stored.
	ServiceWhitelist = "com.amiwrpremium.macontrol.whitelist" // #nosec G101 -- not a credential
)

// ErrNotFound is the typed sentinel returned by [Client.Get] /
// [Client.Delete] when the requested service/account pair does
// not exist in the user's keychain. Distinguishing this from a
// transport error lets callers (e.g. the setup wizard checking
// "is the bot already configured?") branch on absence without
// treating it as a hard failure.
var ErrNotFound = errors.New("keychain: item not found")

// ErrLocked is the typed sentinel returned by [Client.Get] when
// macOS refuses non-interactive access because the user's login
// keychain is locked. The most common cause is a launchd-fired
// daemon starting before the user has finished logging in.
// [Client.Get] retries internally before surfacing this; callers
// that see it should usually wait for login and retry.
var ErrLocked = errors.New("keychain: user interaction not allowed")

// Client is the [runner]-backed wrapper around the macOS
// security(1) CLI. One instance per process is plenty; the type
// is stateless across calls.
//
// Lifecycle:
//   - Constructed once via [New] at daemon startup (or in the
//     setup wizard / whitelist CLI) and shared by every
//     consumer.
//
// Concurrency:
//   - Stateless; safe for concurrent calls from any goroutine
//     (the underlying [runner.Runner] is itself concurrent-safe).
//
// Field roles:
//   - r is the subprocess boundary; tests inject [runner.NewFake]
//     here.
type Client struct {
	// r is the [runner.Runner] used to shell out to security(1).
	r runner.Runner
}

// New returns a [Client] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] (with pre-registered rules)
// in tests.
func New(r runner.Runner) *Client { return &Client{r: r} }

// Set inserts or updates a generic-password entry. The entry is
// keyed by (service, account); calling Set again with the same
// pair overwrites the prior value via security(1)'s `-U` flag.
//
// Behavior:
//   - Builds an `add-generic-password` invocation with `-U` so
//     the call works on first-time creation and subsequent
//     updates alike.
//   - For every path in trustedBinaries, appends a `-T <path>`
//     so that binary can read the entry without prompting the
//     user. Always appends `/usr/bin/security` after the
//     caller-supplied list so CLI subcommands like
//     `macontrol whitelist add` (which themselves shell out to
//     `security`) can also read silently.
//   - Wraps any subprocess failure as
//     "keychain: set <service>/<account>: <err>".
//
// Returns nil on success or the wrapped error otherwise.
func (c *Client) Set(ctx context.Context, service, account, value string, trustedBinaries ...string) error {
	allTrusted := make([]string, 0, len(trustedBinaries)+1)
	allTrusted = append(allTrusted, trustedBinaries...)
	allTrusted = append(allTrusted, "/usr/bin/security")
	args := make([]string, 0, 8+2*len(allTrusted))
	args = append(args,
		"add-generic-password",
		"-s", service,
		"-a", account,
		"-w", value,
		"-U", // update if exists
	)
	for _, t := range allTrusted {
		args = append(args, "-T", t)
	}
	if _, err := c.r.Exec(ctx, "security", args...); err != nil {
		return fmt.Errorf("keychain: set %s/%s: %w", service, account, err)
	}
	return nil
}

// Get retrieves the password for (service, account), retrying
// briefly when the keychain is locked.
//
// Behavior:
//   - Calls security `find-generic-password -s <service> -a <account> -w`.
//     The `-w` flag prints just the password to stdout, no
//     metadata.
//   - Trims a trailing newline from stdout (security always
//     emits one) before returning.
//
// Routing rules (first match wins):
//  1. Subprocess succeeds → returns (trimmed-stdout, nil).
//  2. Subprocess fails with the canonical "not found" message
//     (matched by [isNotFound]) → returns ("", [ErrNotFound]).
//  3. Subprocess fails with the canonical "user interaction not
//     allowed" message (matched by [isLocked]) → records
//     [ErrLocked] as lastErr and retries up to maxAttempts
//     (default 3) with a 5s backoff between attempts. If ctx
//     fires during the backoff, returns ("", ctx.Err())
//     immediately.
//  4. Any other subprocess failure → returns the wrapped error
//     verbatim, no retry.
//
// On exhausted retries the function returns ("", [ErrLocked]).
func (c *Client) Get(ctx context.Context, service, account string) (string, error) {
	const maxAttempts = 3
	const backoff = 5 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := c.r.Exec(ctx,
			"security", "find-generic-password",
			"-s", service, "-a", account, "-w",
		)
		if err == nil {
			return strings.TrimRight(string(out), "\n"), nil
		}
		// Inspect the underlying stderr for the canonical "not found" /
		// "locked" markers so we can surface them as typed errors.
		switch {
		case isNotFound(err):
			return "", ErrNotFound
		case isLocked(err):
			lastErr = ErrLocked
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(backoff):
				}
				continue
			}
		default:
			return "", fmt.Errorf("keychain: get %s/%s: %w", service, account, err)
		}
	}
	return "", lastErr
}

// Delete removes the entry keyed by (service, account).
//
// Behavior:
//   - Calls security `delete-generic-password -s <service> -a <account>`.
//   - Returns nil on success.
//   - Returns [ErrNotFound] when the entry doesn't exist (matched
//     via [isNotFound]).
//   - Wraps any other subprocess failure as
//     "keychain: delete <service>/<account>: <err>".
func (c *Client) Delete(ctx context.Context, service, account string) error {
	_, err := c.r.Exec(ctx,
		"security", "delete-generic-password",
		"-s", service, "-a", account,
	)
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return ErrNotFound
	}
	return fmt.Errorf("keychain: delete %s/%s: %w", service, account, err)
}

// isNotFound reports whether err is a [runner.Error] whose
// captured streams contain the canonical security(1) "not found"
// message.
//
// The security CLI empirically exits with code 44 for missing
// items but the package matches on the message text instead of
// the exit code so the implementation stays portable across
// macOS releases (Apple has historically renumbered exit codes
// without notice).
func isNotFound(err error) bool {
	var rerr *runner.Error
	if errors.As(err, &rerr) {
		body := string(rerr.Stderr) + string(rerr.Stdout)
		if strings.Contains(body, "could not be found in the keychain") ||
			strings.Contains(body, "specified item could not be found") {
			return true
		}
	}
	return false
}

// isLocked reports whether err is a [runner.Error] whose captured
// streams contain the canonical "user interaction not allowed"
// message that macOS emits when the login keychain is locked and
// the caller has no TTY for the unlock prompt.
//
// Two distinct phrasings exist across macOS versions; the
// package matches on either to stay forward-compatible.
func isLocked(err error) bool {
	var rerr *runner.Error
	if errors.As(err, &rerr) {
		body := string(rerr.Stderr) + string(rerr.Stdout)
		if strings.Contains(body, "User interaction is not allowed") ||
			strings.Contains(body, "interaction not allowed") {
			return true
		}
	}
	return false
}
