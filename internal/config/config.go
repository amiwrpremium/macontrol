// Package config loads runtime secrets from the macOS Keychain.
//
// The bot token and user whitelist live exclusively in Keychain
// entries written by `macontrol setup`. There is no .env file, no
// config file, and no env-var override — if the Keychain doesn't
// have both entries, the daemon refuses to start and tells the user
// to run the wizard.
//
// Non-secret runtime config (log level, log file) is passed to the
// daemon as CLI flags on `macontrol run`; see cmd/macontrol.
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

// Config is the typed runtime configuration resolved from the Keychain.
type Config struct {
	TelegramBotToken string
	AllowedUserIDs   []int64
}

// DefaultLogPath returns the macOS-idiomatic log path under
// ~/Library/Logs/macontrol/. Creates the directory with 0750
// permissions if it doesn't exist. The daemon's default
// --log-file value calls this.
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

// Loader bundles the inputs Load needs. Use NewDefaultLoader() in
// production; hand-roll for tests.
type Loader struct {
	Keychain *keychain.Client
	// Account is the Keychain account name (typically the Unix user).
	Account string
}

// NewDefaultLoader returns a Loader wired with a real keychain.Client
// backed by the production runner and the current Unix user.
func NewDefaultLoader() *Loader {
	return &Loader{
		Keychain: keychain.New(runner.New()),
		Account:  currentUser(),
	}
}

// Load is the package-level convenience wrapper.
func Load() (Config, error) { return NewDefaultLoader().Load() }

// Load fetches the bot token and whitelist from the Keychain and
// validates them. Returns a friendly error pointing at `macontrol
// setup` when either secret is missing.
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

// fetch wraps keychain.Get with typed errors that mention the Keychain
// service so users know which entry is missing.
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

// serviceLabel maps a Keychain service name to the human-facing label
// used in error messages.
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

// FormatUserIDs renders a slice of user IDs as the comma-separated
// form stored in the Keychain whitelist entry.
func FormatUserIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// ParseUserIDs is the exported counterpart to parseUserIDs.
func ParseUserIDs(raw string) ([]int64, error) { return parseUserIDs(raw) }

func missingError(field string) error {
	return fmt.Errorf(
		"missing %s in Keychain.\nRun `macontrol setup` to write it",
		field,
	)
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}
