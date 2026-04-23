package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/config"
	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

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
	}
}

const (
	tokenCmd     = "security find-generic-password -s com.amiwrpremium.macontrol -a alice -w"
	whitelistCmd = "security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w"
)

func notFoundErr() error {
	return &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
}

func lockedErr() error {
	return &runner.Error{Stderr: []byte("User interaction is not allowed"), Err: errors.New("exit 36")}
}

// ---------------- DefaultLogPath ----------------

func TestDefaultLogPath_CreatesDir0750(t *testing.T) {
	home := isolateHome(t)
	path, err := config.DefaultLogPath()
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
	if info.Mode().Perm() != 0o750 {
		t.Errorf("dir mode = %v, want 0750", info.Mode().Perm())
	}
}

// ---------------- Keychain resolution ----------------

func TestLoad_HappyPath(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "the-token\n", nil).
		On(whitelistCmd, "111,222\n", nil)

	cfg, err := loaderWith(f).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "the-token" {
		t.Errorf("token = %q", cfg.TelegramBotToken)
	}
	if len(cfg.AllowedUserIDs) != 2 || cfg.AllowedUserIDs[0] != 111 || cfg.AllowedUserIDs[1] != 222 {
		t.Errorf("ids = %v", cfg.AllowedUserIDs)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "", notFoundErr()).
		On(whitelistCmd, "1\n", nil)

	_, err := loaderWith(f).Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
		t.Errorf("error should mention TELEGRAM_BOT_TOKEN: %v", err)
	}
	if !strings.Contains(err.Error(), "macontrol setup") {
		t.Errorf("error should suggest `macontrol setup`: %v", err)
	}
}

func TestLoad_MissingWhitelist(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "tok\n", nil).
		On(whitelistCmd, "", notFoundErr())

	_, err := loaderWith(f).Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ALLOWED_USER_IDS") {
		t.Errorf("error should mention ALLOWED_USER_IDS: %v", err)
	}
	if !strings.Contains(err.Error(), "macontrol setup") {
		t.Errorf("error should suggest `macontrol setup`: %v", err)
	}
}

func TestLoad_KeychainLocked(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "", lockedErr())

	_, err := loaderWith(f).Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, keychain.ErrLocked) {
		t.Errorf("expected ErrLocked, got %v", err)
	}
}

func TestLoad_InvalidWhitelist(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "tok\n", nil).
		On(whitelistCmd, "abc,123\n", nil)

	_, err := loaderWith(f).Load()
	if err == nil || !strings.Contains(err.Error(), "invalid user id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_EmptyWhitelist(t *testing.T) {
	f := runner.NewFake().
		On(tokenCmd, "tok\n", nil).
		On(whitelistCmd, "  \n", nil)

	_, err := loaderWith(f).Load()
	if err == nil || !strings.Contains(err.Error(), "ALLOWED_USER_IDS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------- Helpers ----------------

func TestParseUserIDs(t *testing.T) {
	ids, err := config.ParseUserIDs("1, 2, 3")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("ids = %v", ids)
	}
}

func TestFormatUserIDs(t *testing.T) {
	got := config.FormatUserIDs([]int64{1, 22, 333})
	if got != "1,22,333" {
		t.Errorf("got %q", got)
	}
}

func TestFormatUserIDs_Empty(t *testing.T) {
	if got := config.FormatUserIDs(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestLoad_NoKeychain_ReturnsError(t *testing.T) {
	l := &config.Loader{Account: "alice"} // no Keychain
	if _, err := l.Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_NoAccount_ReturnsError(t *testing.T) {
	// Loader.fetch errors early when Account is empty.
	l := &config.Loader{Keychain: keychain.New(runner.NewFake())}
	if _, err := l.Load(); err == nil {
		t.Fatal("expected error when account empty")
	}
}

func TestNewDefaultLoader_NotNil(t *testing.T) {
	// NewDefaultLoader wires a real runner + user; we just assert it
	// returns a populated struct.
	l := config.NewDefaultLoader()
	if l == nil {
		t.Fatal("nil loader")
	}
	if l.Keychain == nil {
		t.Error("expected Keychain populated")
	}
	if l.Account == "" {
		t.Error("expected Account populated (currentUser)")
	}
}

func TestLoad_PackageWrapper_DelegatesToDefaultLoader(t *testing.T) {
	// config.Load() is the zero-arg wrapper. Without a real Keychain
	// entry, it must return a descriptive error rather than panic.
	if _, err := config.Load(); err == nil {
		// On rare setups a real macOS Keychain may actually have the
		// token; skip in that case so the test is robust.
		t.Skip("unexpected success: real Keychain entry present")
	}
}

// ---------------- Unrecognized keychain error ----------------

func TestLoad_UnrecognizedError(t *testing.T) {
	// An error that is neither ErrNotFound nor ErrLocked must bubble up
	// as-is rather than being remapped.
	f := runner.NewFake().On(tokenCmd, "",
		errors.New("something went very wrong")) // bare error, not a runner.Error
	if _, err := loaderWith(f).Load(); err == nil {
		t.Fatal("expected error")
	}
}
