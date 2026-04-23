package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestExec_Success(t *testing.T) {
	t.Parallel()
	r := runner.New()
	out, err := r.Exec(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "hello\n" {
		t.Fatalf("unexpected stdout: %q", out)
	}
}

func TestExecCombined_MergesStreams(t *testing.T) {
	t.Parallel()
	r := runner.New()
	// Write to stdout AND stderr; assert both end up in the result.
	out, err := r.ExecCombined(context.Background(), "sh", "-c",
		"echo to-stdout; echo to-stderr >&2; exit 0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(out)
	for _, want := range []string{"to-stdout", "to-stderr"} {
		if !strings.Contains(got, want) {
			t.Errorf("merged output missing %q; got %q", want, got)
		}
	}
}

func TestExec_FailureCapturesStderr(t *testing.T) {
	t.Parallel()
	r := runner.New()
	// `false` exits 1 with no output; use `sh -c` to emit to stderr.
	_, err := r.Exec(context.Background(), "sh", "-c", "echo boom >&2; exit 3")
	if err == nil {
		t.Fatal("expected error")
	}
	var rerr *runner.Error
	if !errors.As(err, &rerr) {
		t.Fatalf("expected *runner.Error, got %T", err)
	}
	if got := string(rerr.Stderr); got != "boom\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExec_Timeout(t *testing.T) {
	t.Parallel()
	r := &runner.Exec{DefaultTimeout: 50 * time.Millisecond}
	start := time.Now()
	_, err := r.Exec(context.Background(), "sleep", "2")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 1*time.Second {
		t.Fatal("runner did not honor its default timeout")
	}
}

func TestExec_ContextDeadlineRespected(t *testing.T) {
	t.Parallel()
	r := runner.New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := r.Exec(ctx, "sleep", "2")
	if err == nil {
		t.Fatal("expected deadline error")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("context deadline not honored")
	}
}

func TestFake_RulesAndCalls(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("pmset -g batt", "charged: 100%", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home", nil)

	out, err := f.Exec(context.Background(), "pmset", "-g", "batt")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "charged: 100%" {
		t.Fatalf("stdout = %q", out)
	}

	_, err = f.Sudo(context.Background(), "networksetup", "-getairportnetwork", "en0")
	if err != nil {
		t.Fatal(err)
	}

	calls := f.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if !calls[1].Sudo {
		t.Fatal("second call should be marked sudo")
	}
}

func TestFake_UnknownCommand(t *testing.T) {
	t.Parallel()
	f := runner.NewFake()
	_, err := f.Exec(context.Background(), "no-such-cmd")
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}
