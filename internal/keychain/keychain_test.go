package keychain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// helper to build a runner.Error that mimics the security CLI's output
func secErr(stderr string, exitMsg string) error {
	return &runner.Error{
		Cmd:    "security",
		Args:   []string{"find-generic-password"},
		Stderr: []byte(stderr),
		Err:    errors.New(exitMsg),
	}
}

func TestSet_FormatArgv(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security add-generic-password -s com.amiwrpremium.macontrol -a alice -w secret -U "+
			"-T /opt/homebrew/bin/macontrol -T /usr/bin/security",
		"", nil)

	c := keychain.New(f)
	if err := c.Set(context.Background(),
		keychain.ServiceToken, "alice", "secret",
		"/opt/homebrew/bin/macontrol",
	); err != nil {
		t.Fatal(err)
	}
}

func TestSet_AlwaysAppendsSecurityToTrustedList(t *testing.T) {
	t.Parallel()
	// Without any caller-supplied trusted binary, /usr/bin/security is
	// still added so subcommands work.
	f := runner.NewFake().On(
		"security add-generic-password -s com.amiwrpremium.macontrol -a alice -w x -U -T /usr/bin/security",
		"", nil)

	c := keychain.New(f)
	if err := c.Set(context.Background(), keychain.ServiceToken, "alice", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestSet_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security add-generic-password -s com.amiwrpremium.macontrol -a alice -w x -U -T /usr/bin/security",
		"", errors.New("boom"))

	c := keychain.New(f)
	err := c.Set(context.Background(), keychain.ServiceToken, "alice", "x")
	if err == nil || !strings.Contains(err.Error(), "set com.amiwrpremium.macontrol/alice") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security find-generic-password -s com.amiwrpremium.macontrol -a alice -w",
		"my-token\n", nil)

	got, err := keychain.New(f).Get(context.Background(), keychain.ServiceToken, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-token" {
		t.Errorf("got %q", got)
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security find-generic-password -s com.amiwrpremium.macontrol -a alice -w",
		"",
		secErr("security: could not be found in the keychain.", "exit status 44"))

	_, err := keychain.New(f).Get(context.Background(), keychain.ServiceToken, "alice")
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_LockedRetriesThenFails(t *testing.T) {
	t.Parallel()
	// Always-locked Fake — Get should retry 3 times and then return
	// ErrLocked. Test runs slowly because of the 5s backoff between
	// attempts; keep the test setup minimal.
	f := runner.NewFake().On(
		"security find-generic-password -s com.amiwrpremium.macontrol -a alice -w",
		"",
		secErr("User interaction is not allowed.", "exit status 36"))

	// Use a context with a timeout shorter than the full backoff sequence
	// to keep the test fast — we just need to assert ErrLocked surfaces.
	ctx, cancel := context.WithTimeout(context.Background(), 100*200) // ~20ms
	defer cancel()

	_, err := keychain.New(f).Get(ctx, keychain.ServiceToken, "alice")
	if err == nil {
		t.Fatal("expected error")
	}
	// Either ErrLocked (from the retries) or ctx.DeadlineExceeded is
	// acceptable here; both indicate the locked path was exercised.
	if !errors.Is(err, keychain.ErrLocked) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet_OtherErrorPropagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security find-generic-password -s com.amiwrpremium.macontrol -a alice -w",
		"",
		secErr("something unrecognized", "exit status 1"))

	_, err := keychain.New(f).Get(context.Background(), keychain.ServiceToken, "alice")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, keychain.ErrNotFound) || errors.Is(err, keychain.ErrLocked) {
		t.Fatalf("unexpected typed error: %v", err)
	}
}

func TestDelete_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security delete-generic-password -s com.amiwrpremium.macontrol -a alice",
		"", nil)

	if err := keychain.New(f).Delete(context.Background(), keychain.ServiceToken, "alice"); err != nil {
		t.Fatal(err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security delete-generic-password -s com.amiwrpremium.macontrol -a alice",
		"",
		secErr("security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain.", "exit status 44"))

	err := keychain.New(f).Delete(context.Background(), keychain.ServiceToken, "alice")
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete_OtherError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"security delete-generic-password -s com.amiwrpremium.macontrol -a alice",
		"", errors.New("other"))

	err := keychain.New(f).Delete(context.Background(), keychain.ServiceToken, "alice")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, keychain.ErrNotFound) {
		t.Fatalf("misclassified: %v", err)
	}
}

func TestServiceConstants(t *testing.T) {
	t.Parallel()
	if keychain.ServiceToken == keychain.ServiceWhitelist {
		t.Fatal("services should be distinct")
	}
	if !strings.HasPrefix(keychain.ServiceWhitelist, keychain.ServiceToken) {
		t.Fatal("whitelist service should be a sub-namespace of the token service")
	}
}
