// Package config loads runtime configuration with a three-tier
// resolution: process env → macOS Keychain → legacy `.env` file.
//
// Secret fields (TELEGRAM_BOT_TOKEN, ALLOWED_USER_IDS) live in the
// Keychain by default. The legacy file path is kept as a backwards-
// compatibility fallback and triggers a one-time migration into
// the Keychain on first use.
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
	"time"

	"github.com/joho/godotenv"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Config is the typed runtime configuration.
type Config struct {
	TelegramBotToken string
	AllowedUserIDs   []int64
	LogLevel         string
	LogFile          string
	ConfigFile       string

	// TokenSource and WhitelistSource record where each secret came
	// from in the resolution chain. Useful for `macontrol doctor` and
	// audit logging. One of: "env", "keychain", "file".
	TokenSource     string
	WhitelistSource string
}

// DefaultConfigPath returns the macOS-idiomatic config path under the
// user's Application Support directory. Creates the directory with
// 0700 permissions if it does not exist. Used both for the legacy
// `.env` file path and for the migration backup.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Application Support", "macontrol")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.env"), nil
}

// DefaultLogPath returns the macOS-idiomatic log path under
// ~/Library/Logs.
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

// Loader bundles the inputs Load needs. The zero value is invalid;
// use NewDefaultLoader() in production or hand-roll for tests.
type Loader struct {
	Keychain *keychain.Client
	// Account is the Keychain account name (typically the Unix user).
	Account string
	// Now is overridable for migration-timestamp determinism in tests.
	Now func() time.Time
	// TrustedBinaries are passed to keychain.Set during migration so
	// the new entries grant silent-read access to the daemon. Defaults
	// to os.Executable() in NewDefaultLoader; tests typically leave nil.
	TrustedBinaries []string
}

// NewDefaultLoader returns a Loader wired with a real keychain.Client
// (backed by the production runner) and the current Unix user.
func NewDefaultLoader() *Loader {
	trusted := []string{}
	if exe, err := os.Executable(); err == nil && exe != "" {
		trusted = append(trusted, exe)
	}
	return &Loader{
		Keychain:        keychain.New(runner.New()),
		Account:         currentUser(),
		Now:             time.Now,
		TrustedBinaries: trusted,
	}
}

// Load is the package-level convenience wrapper. Equivalent to
// NewDefaultLoader().Load().
func Load() (Config, error) { return NewDefaultLoader().Load() }

// Load resolves the configuration in the order:
//  1. Process environment (highest priority — useful for dev / CI override)
//  2. macOS Keychain (default for installed users)
//  3. Legacy `.env` file (backwards-compat fallback; triggers auto-migration)
//
// Returns a friendly error pointing at `macontrol setup` when required
// secrets are absent from every source.
func (l *Loader) Load() (Config, error) {
	ctx := context.Background()

	// Step 1: env snapshot (the highest tier).
	envToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	envIDsRaw := os.Getenv("ALLOWED_USER_IDS")

	// Step 2: resolve which legacy file to honor (if any). Explicit
	// MACONTROL_CONFIG wins; otherwise fall back to the default macOS
	// path if it exists.
	path := os.Getenv("MACONTROL_CONFIG")
	fileExplicit := path != ""
	if path == "" {
		def, err := DefaultConfigPath()
		if err == nil {
			if _, statErr := os.Stat(def); statErr == nil {
				path = def
			}
		}
	}

	// Step 3: read file directly (without polluting process env).
	var (
		fileToken  string
		fileIDsRaw string
		fileLoaded bool
	)
	if path != "" {
		if _, statErr := os.Stat(path); statErr == nil { // #nosec G304,G703 -- caller-supplied config path is trusted
			m, readErr := godotenv.Read(path)
			if readErr != nil {
				if fileExplicit {
					return Config{}, fmt.Errorf("read config %s: %w", path, readErr)
				}
				// Best-effort for the default path.
			} else {
				fileToken = m["TELEGRAM_BOT_TOKEN"]
				fileIDsRaw = m["ALLOWED_USER_IDS"]
				fileLoaded = fileToken != "" || fileIDsRaw != ""
			}
		}
	}

	// Step 4: resolve token via env > keychain > file.
	cfg := Config{LogLevel: "info"}
	if envToken != "" {
		cfg.TelegramBotToken = envToken
		cfg.TokenSource = "env"
	} else if l.Keychain != nil && l.Account != "" {
		t, err := l.Keychain.Get(ctx, keychain.ServiceToken, l.Account)
		switch {
		case err == nil:
			cfg.TelegramBotToken = t
			cfg.TokenSource = "keychain"
		case errors.Is(err, keychain.ErrNotFound):
			// fall through to file
		case errors.Is(err, keychain.ErrLocked):
			return Config{}, fmt.Errorf("keychain is locked; the daemon may need to wait for login: %w", err)
		default:
			return Config{}, err
		}
	}
	if cfg.TelegramBotToken == "" && fileToken != "" {
		cfg.TelegramBotToken = fileToken
		cfg.TokenSource = "file"
	}

	// Step 5: resolve whitelist via env > keychain > file.
	if envIDsRaw != "" {
		ids, err := parseUserIDs(envIDsRaw)
		if err != nil {
			return Config{}, fmt.Errorf("ALLOWED_USER_IDS: %w", err)
		}
		cfg.AllowedUserIDs = ids
		cfg.WhitelistSource = "env"
	} else if l.Keychain != nil && l.Account != "" {
		raw, err := l.Keychain.Get(ctx, keychain.ServiceWhitelist, l.Account)
		switch {
		case err == nil:
			ids, parseErr := parseUserIDs(raw)
			if parseErr != nil {
				return Config{}, fmt.Errorf("keychain whitelist value: %w", parseErr)
			}
			cfg.AllowedUserIDs = ids
			cfg.WhitelistSource = "keychain"
		case errors.Is(err, keychain.ErrNotFound):
			// fall through to file
		case errors.Is(err, keychain.ErrLocked):
			return Config{}, fmt.Errorf("keychain is locked; the daemon may need to wait for login: %w", err)
		default:
			return Config{}, err
		}
	}
	if len(cfg.AllowedUserIDs) == 0 && fileIDsRaw != "" {
		ids, err := parseUserIDs(fileIDsRaw)
		if err != nil {
			return Config{}, fmt.Errorf("legacy file ALLOWED_USER_IDS: %w", err)
		}
		cfg.AllowedUserIDs = ids
		cfg.WhitelistSource = "file"
	}

	// Optional non-secret env vars.
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MACONTROL_LOG"); v != "" {
		cfg.LogFile = v
	}

	// Step 6: validation.
	missing := []string{}
	if cfg.TelegramBotToken == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if len(cfg.AllowedUserIDs) == 0 {
		missing = append(missing, "ALLOWED_USER_IDS")
	}
	if len(missing) > 0 {
		return Config{}, missingError(missing)
	}

	// Step 7: auto-migration. If we ended up using file-sourced secrets
	// for at least one of the two, push them into the Keychain and
	// rename the file as a backup.
	if fileLoaded && (cfg.TokenSource == "file" || cfg.WhitelistSource == "file") {
		l.migrate(ctx, path, cfg)
	}

	if cfg.LogFile == "" {
		if lf, err := DefaultLogPath(); err == nil {
			cfg.LogFile = lf
		}
	}
	cfg.ConfigFile = path
	return cfg, nil
}

// migrate copies file-sourced secrets into the Keychain (where missing)
// and renames the legacy file to `<path>.migrated.<unix-ts>`. Failures
// are logged via stderr but never abort startup — defense in depth.
func (l *Loader) migrate(ctx context.Context, path string, cfg Config) {
	if l.Keychain == nil || l.Account == "" {
		return
	}

	wrote := false
	if cfg.TokenSource == "file" && cfg.TelegramBotToken != "" {
		if err := l.Keychain.Set(ctx, keychain.ServiceToken, l.Account, cfg.TelegramBotToken, l.TrustedBinaries...); err != nil {
			fmt.Fprintf(os.Stderr, "macontrol: keychain migration of token failed: %v\n", err)
			return
		}
		wrote = true
	}
	if cfg.WhitelistSource == "file" && len(cfg.AllowedUserIDs) > 0 {
		if err := l.Keychain.Set(ctx, keychain.ServiceWhitelist, l.Account, formatUserIDs(cfg.AllowedUserIDs), l.TrustedBinaries...); err != nil {
			fmt.Fprintf(os.Stderr, "macontrol: keychain migration of whitelist failed: %v\n", err)
			return
		}
		wrote = true
	}
	if !wrote {
		return
	}

	ts := l.Now().Unix()
	backup := fmt.Sprintf("%s.migrated.%d", path, ts)
	if err := os.Rename(path, backup); err != nil { // #nosec G304,G703 -- migration target derived from trusted config path
		fmt.Fprintf(os.Stderr, "macontrol: keychain migration succeeded but renaming %s failed: %v\n", path, err)
		return
	}
	fmt.Fprintf(os.Stderr, "macontrol: migrated config from .env to Keychain (backup at %s)\n", backup)
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

// FormatUserIDs renders a slice of user IDs as the comma-separated form
// stored in Keychain and accepted in env vars.
func FormatUserIDs(ids []int64) string { return formatUserIDs(ids) }

func formatUserIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// ParseUserIDs is the exported counterpart to formatUserIDs.
func ParseUserIDs(raw string) ([]int64, error) { return parseUserIDs(raw) }

func missingError(fields []string) error {
	return fmt.Errorf(
		"missing required config: %s.\nRun `macontrol setup` to write them to the Keychain",
		strings.Join(fields, ", "),
	)
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}
