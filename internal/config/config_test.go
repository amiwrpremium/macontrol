package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/config"
	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

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

func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func loaderWith(f *runner.Fake) *config.Loader {
	return &config.Loader{
		Keychain: keychain.New(f),
		Account:  "alice",
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
	}
}

func writeEnvFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ---------------- DefaultConfigPath / DefaultLogPath ----------------

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
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestDefaultLogPath_CreatesDir0750(t *testing.T) {
	_ = isolateHome(t)
	path, err := config.DefaultLogPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("dir does not exist: %v", err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Errorf("dir mode = %v, want 0750", info.Mode().Perm())
	}
}

// ---------------- Three-tier resolution ----------------

func TestLoad_EnvHasHighestPriority(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")
	t.Setenv("TELEGRAM_BOT_TOKEN", "from-env")
	t.Setenv("ALLOWED_USER_IDS", "111,222")

	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "from-keychain\n", nil).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "999,888\n", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "from-env" {
		t.Errorf("token = %q (TokenSource=%q)", cfg.TelegramBotToken, cfg.TokenSource)
	}
	if len(cfg.AllowedUserIDs) != 2 || cfg.AllowedUserIDs[0] != 111 {
		t.Errorf("ids = %v", cfg.AllowedUserIDs)
	}
	if cfg.TokenSource != "env" || cfg.WhitelistSource != "env" {
		t.Errorf("sources = %s/%s", cfg.TokenSource, cfg.WhitelistSource)
	}
}

func TestLoad_KeychainWhenNoEnv(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "from-keychain\n", nil).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "111,222\n", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "from-keychain" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
	if cfg.TokenSource != "keychain" || cfg.WhitelistSource != "keychain" {
		t.Errorf("sources = %s/%s", cfg.TokenSource, cfg.WhitelistSource)
	}
}

func TestLoad_FileFallback_AndAutoMigration(t *testing.T) {
	home := isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	cfgPath := filepath.Join(home, "Library", "Application Support", "macontrol", "config.env")
	writeEnvFile(t, cfgPath, "TELEGRAM_BOT_TOKEN=from-file\nALLOWED_USER_IDS=333\n")

	notFound := &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "", notFound).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "", notFound).
		On("security add-generic-password -s com.amiwrpremium.macontrol -a alice -w from-file -U -T /usr/bin/security", "", nil).
		On("security add-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w 333 -U -T /usr/bin/security", "", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "from-file" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
	if cfg.TokenSource != "file" || cfg.WhitelistSource != "file" {
		t.Errorf("sources = %s/%s", cfg.TokenSource, cfg.WhitelistSource)
	}

	// Original file should be renamed
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("expected original file to be renamed; stat err = %v", err)
	}
	// Backup with the canned timestamp from loaderWith() should exist
	expectedBackup := cfgPath + ".migrated.1700000000"
	if _, err := os.Stat(expectedBackup); err != nil {
		t.Errorf("expected backup at %s, stat err = %v", expectedBackup, err)
	}
}

func TestLoad_NoMigration_WhenKeychainAlreadyHasValues(t *testing.T) {
	home := isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	cfgPath := filepath.Join(home, "Library", "Application Support", "macontrol", "config.env")
	writeEnvFile(t, cfgPath, "TELEGRAM_BOT_TOKEN=stale-file\nALLOWED_USER_IDS=999\n")

	// Keychain wins; the file is treated as legacy crud and not
	// migrated (because the keychain already has the entries).
	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "fresh-token\n", nil).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "100\n", nil)

	if _, err := loaderWith(f).Load(); err != nil {
		t.Fatal(err)
	}
	// File should still exist (no migration)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("file should still exist (no migration); stat err = %v", err)
	}
}

func TestLoad_MissingEverywhere_FriendlyError(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	notFound := &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "", notFound).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "", notFound)

	_, err := loaderWith(f).Load()
	if err == nil {
		t.Fatal("expected missing-config error")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
		t.Errorf("error should mention TELEGRAM_BOT_TOKEN: %v", err)
	}
	if !strings.Contains(err.Error(), "macontrol setup") {
		t.Errorf("error should suggest `macontrol setup`: %v", err)
	}
}

func TestLoad_LogLevelDefault(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "LOG_LEVEL")

	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "tok\n", nil).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "1\n", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("log level = %q", cfg.LogLevel)
	}
}

func TestLoad_InvalidWhitelistInKeychain(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "MACONTROL_CONFIG", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "tok\n", nil).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "abc,123\n", nil)

	_, err := loaderWith(f).Load()
	if err == nil || !strings.Contains(err.Error(), "invalid user id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ExplicitMACONTROL_CONFIG(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")

	dir := t.TempDir()
	custom := filepath.Join(dir, "alt.env")
	writeEnvFile(t, custom, "TELEGRAM_BOT_TOKEN=alt-tok\nALLOWED_USER_IDS=7\n")
	t.Setenv("MACONTROL_CONFIG", custom)

	notFound := &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "", notFound).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "", notFound).
		On("security add-generic-password -s com.amiwrpremium.macontrol -a alice -w alt-tok -U -T /usr/bin/security", "", nil).
		On("security add-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w 7 -U -T /usr/bin/security", "", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "alt-tok" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
}

func TestLoad_ExplicitMACONTROL_CONFIG_MissingFile(t *testing.T) {
	_ = isolateHome(t)
	unsetEnv(t, "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS")
	t.Setenv("MACONTROL_CONFIG", "/nonexistent/path/cfg.env")

	notFound := &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
	f := runner.NewFake().
		On("security find-generic-password -s com.amiwrpremium.macontrol -a alice -w", "", notFound).
		On("security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w", "", notFound)

	_, err := loaderWith(f).Load()
	if err == nil {
		t.Fatal("expected error (missing config)")
	}
}
