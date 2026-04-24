// Package config loads runtime secrets from the macOS Keychain.
//
// macontrol is intentionally Keychain-only: there is no .env file,
// no on-disk config file, and no env-var override for the bot token
// or the user-ID whitelist. Both live in dedicated Keychain entries
// written by the `macontrol setup` wizard. If either entry is
// missing or empty, the daemon refuses to start with a friendly
// error pointing at the wizard.
//
// Non-secret runtime config (log level, log file path) is passed
// to the daemon as CLI flags on `macontrol run` instead of going
// through this package — see cmd/macontrol/daemon.go.
//
// The package surface is small:
//
//   - [Config] is the resolved value handed to the bot.
//   - [Load] / [NewDefaultLoader] / [Loader] are the production
//     entry points.
//   - [DefaultLogPath] computes the macOS-idiomatic default for the
//     daemon's --log-file flag.
//   - [ParseUserIDs] / [FormatUserIDs] are exported helpers shared
//     with cmd/macontrol/whitelist.go for the `macontrol whitelist`
//     subcommand.
package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Config is the typed runtime configuration the daemon needs to
// boot. Resolved entirely from the Keychain by [Loader.Load];
// callers receive a fully-validated value (no empty token, at
// least one whitelisted user) or a friendly error.
//
// Lifecycle:
//   - Constructed once at daemon startup and stashed for the
//     process lifetime; never mutated post-load.
//
// Field roles:
//   - TelegramBotToken is the secret @BotFather token. The bot
//     package passes it to bot.Start.
//   - AllowedUserIDs is the parsed whitelist; the bot's auth
//     middleware compares each incoming update's sender ID
//     against this slice (membership, not range).
type Config struct {
	// TelegramBotToken is the secret @BotFather token, populated
	// from the [keychain.ServiceToken] entry.
	TelegramBotToken string

	// AllowedUserIDs is the parsed whitelist of Telegram user
	// IDs permitted to interact with the bot, populated from the
	// [keychain.ServiceWhitelist] entry.
	AllowedUserIDs []int64
}

// DefaultLogPath returns the macOS-idiomatic log file path under
// ~/Library/Logs/macontrol/ and ensures the parent directory
// exists.
//
// Behavior:
//   - Resolves the home directory via [os.UserHomeDir]; returns
//     the underlying error verbatim on failure (rare; happens on
//     hosts with $HOME unset).
//   - Creates the macontrol/ subdirectory with mode 0o750 if
//     missing; pre-existing directories are left untouched.
//   - Returns "<home>/Library/Logs/macontrol/macontrol.log".
//
// The daemon's --log-file flag default calls this; see
// cmd/macontrol/daemon.go.
func DefaultLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "macontrol")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return filepath.Join(dir, "macontrol.log"), nil
}

// Loader bundles the inputs [Loader.Load] needs to resolve a
// [Config] from the Keychain. Production callers use
// [NewDefaultLoader]; tests construct directly with a fake
// [keychain.Client].
//
// Lifecycle:
//   - Constructed once at daemon startup, used to call Load,
//     then discarded.
//
// Field roles:
//   - Keychain is the client that does the actual Get; tests
//     inject a [runner.Fake]-backed instance.
//   - Account is the macOS Keychain account name — typically
//     the Unix username from [currentUser]. All macontrol
//     entries are written under this account so the daemon's
//     -a query matches.
type Loader struct {
	// Keychain is the client used to read the token and
	// whitelist entries.
	Keychain *keychain.Client

	// Account is the Keychain account name (typically the Unix
	// user) under which both entries live.
	Account string
}

// NewDefaultLoader returns a [Loader] wired with a real
// keychain.Client backed by [runner.New] and the current Unix
// user as the Keychain account. Used by the daemon at boot.
func NewDefaultLoader() *Loader {
	return &Loader{
		Keychain: keychain.New(runner.New()),
		Account:  currentUser(),
	}
}

// Load is the package-level convenience wrapper that calls
// [NewDefaultLoader] then [Loader.Load]. Used by daemon main when
// no test customisation is needed.
func Load() (Config, error) { return NewDefaultLoader().Load() }

// Load fetches the bot token and the whitelist from the Keychain,
// validates both, and returns the resolved [Config].
//
// Behavior:
//  1. Fetches keychain.ServiceToken via [Loader.fetch]; missing
//     returns a friendly error mentioning TELEGRAM_BOT_TOKEN.
//  2. Fetches keychain.ServiceWhitelist via [Loader.fetch];
//     missing returns a friendly error mentioning
//     ALLOWED_USER_IDS.
//  3. Parses the whitelist via [parseUserIDs]; on parse error
//     wraps as "keychain whitelist value: <err>".
//  4. Rejects an empty parsed list with the same
//     ALLOWED_USER_IDS-missing error so the user sees one
//     consistent message regardless of whether the entry was
//     unset or set to "".
//
// Returns the populated [Config] on success.
func (l *Loader) Load() (Config, error) {
	ctx := context.Background()

	token, err := l.fetch(ctx, keychain.ServiceToken)
	if err != nil {
		return Config{}, err
	}

	raw, err := l.fetch(ctx, keychain.ServiceWhitelist)
	if err != nil {
		return Config{}, err
	}

	ids, err := parseUserIDs(raw)
	if err != nil {
		return Config{}, fmt.Errorf("keychain whitelist value: %w", err)
	}
	if len(ids) == 0 {
		return Config{}, missingError("ALLOWED_USER_IDS")
	}

	return Config{TelegramBotToken: token, AllowedUserIDs: ids}, nil
}

// fetch wraps [keychain.Client.Get] with typed errors that name
// the missing entry in human-facing terms.
//
// Routing rules (first match wins):
//  1. Loader has no Keychain client or no Account → returns
//     "config: loader has no keychain client or account"
//     (programmer error; tests forgot to initialise the loader).
//  2. Keychain Get succeeds with non-empty value → returns
//     (value, nil).
//  3. Keychain Get succeeds with empty value → treats as missing,
//     returns the friendly TELEGRAM_BOT_TOKEN/ALLOWED_USER_IDS
//     "missing in Keychain" error.
//  4. Keychain Get returns ErrNotFound → same friendly error.
//  5. Keychain Get returns ErrLocked → wraps as "keychain is
//     locked; the daemon may need to wait for login".
//  6. Any other Keychain error → returned verbatim.
func (l *Loader) fetch(ctx context.Context, service string) (string, error) {
	if l.Keychain == nil || l.Account == "" {
		return "", errors.New("config: loader has no keychain client or account")
	}
	v, err := l.Keychain.Get(ctx, service, l.Account)
	switch {
	case err == nil:
		if v == "" {
			return "", missingError(serviceLabel(service))
		}
		return v, nil
	case errors.Is(err, keychain.ErrNotFound):
		return "", missingError(serviceLabel(service))
	case errors.Is(err, keychain.ErrLocked):
		return "", fmt.Errorf("keychain is locked; the daemon may need to wait for login: %w", err)
	default:
		return "", err
	}
}

// serviceLabel maps a [keychain] service constant to the
// human-facing variable label used in error messages
// (TELEGRAM_BOT_TOKEN / ALLOWED_USER_IDS). Falls through to the
// raw service name for anything unrecognised so a future caller
// can't accidentally rename an error message into something
// invisible.
func serviceLabel(service string) string {
	switch service {
	case keychain.ServiceToken:
		return "TELEGRAM_BOT_TOKEN"
	case keychain.ServiceWhitelist:
		return "ALLOWED_USER_IDS"
	default:
		return service
	}
}

// parseUserIDs splits the comma-separated whitelist raw string
// into a slice of int64 user IDs. Whitespace around each entry
// is trimmed; empty entries (e.g. trailing comma) are skipped.
//
// Behavior:
//   - Returns the parsed slice on success (may be empty if every
//     entry was blank).
//   - Returns ("", error) on the first non-numeric entry,
//     wrapping strconv.ParseInt's error and naming the offending
//     token.
func parseUserIDs(raw string) ([]int64, error) {
	out := []int64{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user id %q: %w", p, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// FormatUserIDs renders a slice of user IDs as the
// comma-separated string stored in the Keychain whitelist entry.
// Inverse of [parseUserIDs] / [ParseUserIDs]. Used by the
// `macontrol whitelist` CLI to write back after add/remove.
func FormatUserIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// ParseUserIDs is the exported counterpart to [parseUserIDs] for
// callers outside the package (cmd/macontrol/whitelist.go reads
// the current whitelist before mutating it).
func ParseUserIDs(raw string) ([]int64, error) { return parseUserIDs(raw) }

// missingError builds the canonical "missing X in Keychain — run
// macontrol setup" error. Used both for "entry not present" and
// "entry present but empty" so the user sees one consistent
// remediation regardless of cause.
func missingError(field string) error {
	return fmt.Errorf(
		"missing %s in Keychain.\nRun `macontrol setup` to write it",
		field,
	)
}

// currentUser returns the Unix username for use as the Keychain
// account. Falls back to $USER if [user.Current] fails (rare;
// happens in stripped containers without /etc/passwd entries).
// Returns "" if both lookups fail; [Loader.fetch] then surfaces
// "config: loader has no keychain client or account".
func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}
