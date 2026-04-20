package runner_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestExec_NonExistentBinary(t *testing.T) {
	t.Parallel()
	_, err := runner.New().Exec(context.Background(), "definitely-not-a-real-command-2026")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExec_EmptyArgs(t *testing.T) {
	t.Parallel()
	out, err := runner.New().Exec(context.Background(), "true")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty stdout, got %q", out)
	}
}

func TestSudo_FakeMarksCall(t *testing.T) {
	t.Parallel()
	// Fake.Sudo records Sudo=true on the call — production Runner.Sudo
	// additionally prepends "sudo -n" to the argv, but the Fake bypasses
	// that (by design, to keep test fixtures readable).
	f := runner.NewFake().On("echo hi", "hi\n", nil)
	out, err := f.Sudo(context.Background(), "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hi\n" {
		t.Fatalf("stdout = %q", out)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[0].Name != "echo" {
		t.Errorf("Fake records the original name; got %q", calls[0].Name)
	}
	if !calls[0].Sudo {
		t.Error("expected Sudo=true recorded")
	}
}

func TestExec_SudoPrependsDashN_RealRunner(t *testing.T) {
	t.Parallel()
	// The real runner wraps the command in `sudo -n <name> <args>`. We
	// can't actually exec sudo successfully in CI without a sudoers entry,
	// so the call fails — but it fails through the sudo binary, which is
	// what we care about. Assert that the error message references sudo.
	_, err := runner.New().Sudo(context.Background(), "definitely-not-a-real-binary-xyz")
	if err == nil {
		t.Skip("sudo -n succeeded unexpectedly; CI has a lenient sudoers entry")
	}
	if !strings.Contains(err.Error(), "sudo") {
		t.Errorf("expected error to mention sudo: %v", err)
	}
}

func TestError_String_WithStderr(t *testing.T) {
	t.Parallel()
	_, err := runner.New().Exec(context.Background(), "sh", "-c", "echo boom >&2; exit 4")
	var rerr *runner.Error
	if !errors.As(err, &rerr) {
		t.Fatalf("expected *runner.Error, got %T", err)
	}
	msg := rerr.Error()
	if !strings.Contains(msg, "boom") {
		t.Errorf("expected stderr in error message: %q", msg)
	}
	if !strings.Contains(msg, "sh") {
		t.Errorf("expected command name in error: %q", msg)
	}
}

func TestError_String_WithStdoutOnly(t *testing.T) {
	t.Parallel()
	_, err := runner.New().Exec(context.Background(), "sh", "-c", "echo hello; exit 1")
	var rerr *runner.Error
	if !errors.As(err, &rerr) {
		t.Fatal("expected runner.Error")
	}
	msg := rerr.Error()
	if !strings.Contains(msg, "hello") {
		t.Errorf("expected stdout in error (no stderr): %q", msg)
	}
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()
	_, err := runner.New().Exec(context.Background(), "false")
	unwrapped := errors.Unwrap(err)
	if unwrapped == nil {
		t.Fatal("expected Unwrap to return inner error")
	}
}

func TestFake_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("echo", "ok", nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.Exec(context.Background(), "echo")
		}()
	}
	wg.Wait()
	if n := len(f.Calls()); n != 50 {
		t.Fatalf("expected 50 recorded calls, got %d", n)
	}
}

func TestFake_PrefixFallback(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kubectl get", "pods output", nil)
	out, err := f.Exec(context.Background(), "kubectl", "get", "pods")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "pods output" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestFake_Calls_IsSnapshot(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("echo", "", nil)
	_, _ = f.Exec(context.Background(), "echo")
	first := f.Calls()
	_, _ = f.Exec(context.Background(), "echo")
	if len(first) != 1 {
		t.Fatalf("snapshot should be frozen at 1, got %d", len(first))
	}
	if len(f.Calls()) != 2 {
		t.Fatalf("live count should be 2, got %d", len(f.Calls()))
	}
}

func TestExec_DefaultTimeoutZero(t *testing.T) {
	t.Parallel()
	r := &runner.Exec{} // DefaultTimeout == 0 — package default applies
	out, err := r.Exec(context.Background(), "echo", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("stdout = %q", out)
	}
}
