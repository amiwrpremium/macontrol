// Package keychain wraps the macOS `security` CLI to read, write, and
// delete generic-password Keychain entries used by macontrol.
//
// The store identifiers are split by service name so each secret can be
// queried or removed independently:
//
//	com.amiwrpremium.macontrol           — bot token
//	com.amiwrpremium.macontrol.whitelist — comma-separated user IDs
//
// Every shell-out goes through runner.Runner so tests can use runner.Fake
// instead of the real `security` binary.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service identifiers used by macontrol. Exposed so callers don't need
// to hardcode the strings. They're bundle-style names, not credentials.
const (
	ServiceToken     = "com.amiwrpremium.macontrol"           // #nosec G101 -- not a credential
	ServiceWhitelist = "com.amiwrpremium.macontrol.whitelist" // #nosec G101 -- not a credential
)

// ErrNotFound is returned when the requested item does not exist in the
// Keychain. Distinct from a transport error so callers can fall through
// to other config sources without treating it as a hard failure.
var ErrNotFound = errors.New("keychain: item not found")

// ErrLocked is returned when the user's login keychain is locked and
// macOS refuses non-interactive access. Callers should typically retry
// after a backoff (Get already retries internally).
var ErrLocked = errors.New("keychain: user interaction not allowed")

// Client wraps the security CLI calls.
type Client struct {
	r runner.Runner
}

// New returns a Client backed by r. Pass runner.New() in production,
// runner.NewFake() in tests.
func New(r runner.Runner) *Client { return &Client{r: r} }

// Set inserts or updates a generic-password entry.
//
// trustedBinaries is the list of executables granted silent-read access
// to this entry via -T. Pass the macontrol binary path so the daemon can
// read without a prompt. /usr/bin/security is added automatically so
// CLI subcommands like `macontrol whitelist add` (which shells out to
// `security`) can also read silently.
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

// Get retrieves an entry, retrying briefly when the keychain is locked
// (common during launchd-triggered boot before login finishes unlocking).
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

// Delete removes an entry. Returns ErrNotFound if it didn't exist.
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

// isNotFound matches the security(1) "could not be found in the
// keychain" error text. Empirically the security CLI exits 44 for
// missing items, but we match on the message to stay portable.
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

// isLocked matches the message macOS returns when the login keychain
// is locked and we're not in an interactive session.
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
