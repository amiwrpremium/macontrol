// Package config loads runtime configuration from the environment (optionally
// seeded by a .env file at the standard macOS path).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the typed runtime configuration. All fields are either read from
// the environment directly or from ~/Library/Application Support/macontrol/config.env.
type Config struct {
	TelegramBotToken string  `env:"TELEGRAM_BOT_TOKEN,required"`
	AllowedUserIDs   []int64 `env:"ALLOWED_USER_IDS,required" envSeparator:","`
	LogLevel         string  `env:"LOG_LEVEL" envDefault:"info"`
	LogFile          string  `env:"MACONTROL_LOG"`
	ConfigFile       string  `env:"MACONTROL_CONFIG"`
}

// DefaultConfigPath returns the macOS-idiomatic config path under the user's
// Application Support directory. Creates the directory with 0o700 permissions
// if it does not exist.
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

// DefaultLogPath returns the macOS-idiomatic log path under ~/Library/Logs.
func DefaultLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "macontrol")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "macontrol.log"), nil
}

// Load reads the config file (if present) into the process environment,
// then parses and validates the Config struct. Missing required fields
// yield a user-friendly error pointing at `macontrol setup`.
func Load() (Config, error) {
	// Determine which env file to honor. Explicit MACONTROL_CONFIG wins,
	// otherwise fall back to the default macOS path if it exists.
	path := os.Getenv("MACONTROL_CONFIG")
	if path == "" {
		def, err := DefaultConfigPath()
		if err == nil {
			if _, statErr := os.Stat(def); statErr == nil {
				path = def
			}
		}
	}
	if path != "" {
		if err := godotenv.Overload(path); err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, friendlyError(err)
	}
	if cfg.LogFile == "" {
		lf, err := DefaultLogPath()
		if err == nil {
			cfg.LogFile = lf
		}
	}
	cfg.ConfigFile = path
	return cfg, nil
}

func friendlyError(err error) error {
	var agg env.AggregateError
	if errors.As(err, &agg) {
		missing := []string{}
		for _, e := range agg.Errors {
			var reqErr env.EnvVarIsNotSetError
			if errors.As(e, &reqErr) {
				missing = append(missing, reqErr.Key)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf(
				"missing required config: %s.\nRun `macontrol setup` to write them, or edit the config file directly",
				strings.Join(missing, ", "),
			)
		}
	}
	return err
}
