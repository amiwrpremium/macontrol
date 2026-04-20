package power_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/power"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestLock(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession -suspend",
		"", nil)
	if err := power.New(f).Lock(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSleep(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pmset sleepnow", "", nil)
	if err := power.New(f).Sleep(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRestart(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		`osascript -e tell application "System Events" to restart`, "", nil)
	if err := power.New(f).Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestShutdown(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		`osascript -e tell application "System Events" to shut down`, "", nil)
	if err := power.New(f).Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLogout(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		`osascript -e tell application "System Events" to log out`, "", nil)
	if err := power.New(f).Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestKeepAwake_RejectsNonPositive(t *testing.T) {
	t.Parallel()
	cases := []time.Duration{0, -1 * time.Second, -1 * time.Hour}
	for _, d := range cases {
		if err := power.New(runner.NewFake()).KeepAwake(context.Background(), d); err == nil {
			t.Errorf("duration %v should be rejected", d)
		}
	}
}

func TestKeepAwake_FormsNohupCommand(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sh -c nohup caffeinate -d -t 60 >/dev/null 2>&1 &", "", nil)
	if err := power.New(f).KeepAwake(context.Background(), time.Minute); err != nil {
		t.Fatal(err)
	}
	// Sanity: command string mentions caffeinate and the seconds count.
	calls := f.Calls()
	if len(calls) == 0 {
		t.Fatal("no call recorded")
	}
	joined := strings.Join(calls[0].Args, " ")
	if !strings.Contains(joined, "caffeinate -d -t 60") {
		t.Fatalf("expected caffeinate 60s in command; got %q", joined)
	}
}

func TestKeepAwake_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sh -c nohup caffeinate -d -t 5 >/dev/null 2>&1 &", "", errors.New("no sh"))
	if err := power.New(f).KeepAwake(context.Background(), 5*time.Second); err == nil {
		t.Fatal("expected error")
	}
}

func TestCancelKeepAwake_NoMatchesIgnored(t *testing.T) {
	t.Parallel()
	// pkill exits 1 when no matches — our code should treat that as success.
	f := runner.NewFake().On("pkill -x caffeinate", "", &runner.Error{
		Cmd: "pkill", Args: []string{"-x", "caffeinate"},
		Err: errors.New("exit status 1"),
	})
	if err := power.New(f).CancelKeepAwake(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCancelKeepAwake_OtherErrorPropagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pkill -x caffeinate", "", &runner.Error{
		Cmd: "pkill", Args: []string{"-x", "caffeinate"},
		Err: errors.New("exit status 2"),
	})
	if err := power.New(f).CancelKeepAwake(context.Background()); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCancelKeepAwake_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pkill -x caffeinate", "", nil)
	if err := power.New(f).CancelKeepAwake(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCancelKeepAwake_PlainErrorPropagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pkill -x caffeinate", "", errors.New("bare"))
	if err := power.New(f).CancelKeepAwake(context.Background()); err == nil {
		t.Fatal("expected propagation of non-runner.Error")
	}
}
