package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/config"
)

// unsetEnv unsets a variable for the duration of the test, restoring its
// prior value (if any) on cleanup. Needed because t.Setenv("X", "") still
// leaves X *set* to an empty string, which caarlos0/env treats as present.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	saved := map[string]string{}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
		}
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			_ = os.Unsetenv(k)
			if v, ok := saved[k]; ok {
				_ = os.Setenv(k, v)
			}
		}
	})
}

// isolateHome sets HOME/USERPROFILE to a temp dir so DefaultConfigPath and
// DefaultLogPath don't leak into the real file system.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestDefaultConfigPath_CreatesDir0700(t *testing.T) {
	home := isolateHome(t)
	path, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, home) {
		t.Fatalf("path %q not under HOME %q", path, home)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("dir does not exist: %v", err)
	}
	// Check mode on Unix; on other platforms we just assert dir exists.
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestDefaultLogPath_CreatesDir0755(t *testing.T) {
	_ = isolateHome(t)
	path, err := config.DefaultLogPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("dir does not exist: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("dir mode = %v, want 0755", info.Mode().Perm())
	}
}

func writeEnv(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_HappyPath(t *testing.T) {
	_ = isolateHome(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	writeEnv(t, envFile, "TELEGRAM_BOT_TOKEN=abc\nALLOWED_USER_IDS=1,2,3\n")

	// Clear any host env that might leak in.
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("MACONTROL_CONFIG", envFile)
	t.Setenv("LOG_LEVEL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "abc" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUserIDs) != 3 || cfg.AllowedUserIDs[0] != 1 || cfg.AllowedUserIDs[2] != 3 {
		t.Errorf("ids = %v", cfg.AllowedUserIDs)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("level = %q", cfg.LogLevel)
	}
	if cfg.ConfigFile != envFile {
		t.Errorf("config file = %q", cfg.ConfigFile)
	}
}

func TestLoad_MissingTokenFriendlyError(t *testing.T) {
	_ = isolateHome(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	writeEnv(t, envFile, "ALLOWED_USER_IDS=1\n")

	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("MACONTROL_CONFIG", envFile)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "macontrol setup") {
		t.Errorf("error should mention `macontrol setup`; got: %v", err)
	}
	if !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
		t.Errorf("error should name the missing field; got: %v", err)
	}
}

func TestLoad_MissingBothFields(t *testing.T) {
	_ = isolateHome(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	writeEnv(t, envFile, "\n")
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("MACONTROL_CONFIG", envFile)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in error: %v", want, err)
		}
	}
}

func TestLoad_DefaultLogLevelInfo(t *testing.T) {
	_ = isolateHome(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	writeEnv(t, envFile, "TELEGRAM_BOT_TOKEN=t\nALLOWED_USER_IDS=1\n")
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("MACONTROL_CONFIG", envFile)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("log level = %q, want info", cfg.LogLevel)
	}
}

func TestLoad_InvalidUserIDsErrors(t *testing.T) {
	_ = isolateHome(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	writeEnv(t, envFile, "TELEGRAM_BOT_TOKEN=t\nALLOWED_USER_IDS=notanumber\n")
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("MACONTROL_CONFIG", envFile)

	if _, err := config.Load(); err == nil {
		t.Fatal("expected parse error for non-integer ids")
	}
}

func TestLoad_NoEnvFile_UsesProcessEnv(t *testing.T) {
	_ = isolateHome(t)
	// Explicitly unset MACONTROL_CONFIG; there should be no default file.
	t.Setenv("MACONTROL_CONFIG", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "from-env")
	t.Setenv("ALLOWED_USER_IDS", "42")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "from-env" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("level = %q", cfg.LogLevel)
	}
	if len(cfg.AllowedUserIDs) != 1 || cfg.AllowedUserIDs[0] != 42 {
		t.Errorf("ids = %v", cfg.AllowedUserIDs)
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	t.Setenv("MACONTROL_CONFIG", "/nonexistent/path/cfg.env")
	t.Setenv("TELEGRAM_BOT_TOKEN", "t")
	t.Setenv("ALLOWED_USER_IDS", "1")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected read error for missing explicit MACONTROL_CONFIG")
	}
}
