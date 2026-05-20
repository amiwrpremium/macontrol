package apps_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/apps"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// runningScriptKey is the runner.Fake lookup key for the
// listing osascript invocation. Mirrors the unexported
// runningScript constant in the apps package — tests
// hard-code the literal so a refactor that changes the script
// fails the test loudly rather than silently.
const runningScriptKey = "osascript -e tell application \"System Events\"\n" +
	"set out to \"\"\n" +
	"repeat with p in (processes whose background only is false)\n" +
	"set out to out & (name of p) & \"|\" & (unix id of p) & \"|\" & ((not (visible of p)) as text) & linefeed\n" +
	"end repeat\n" +
	"return out\n" +
	"end tell"

// fakeRunning returns a runner.Fake stubbed to return canned
// stdout for the listing osascript.
func fakeRunning(canned string) *runner.Fake {
	return runner.NewFake().On(runningScriptKey, canned, nil)
}

func TestRunning_FullListingSortsAlphabetical(t *testing.T) {
	t.Parallel()
	stdout := "Safari|1234|false\nMail|2345|false\nVisual Studio Code|3456|false\n"
	got, err := apps.New(fakeRunning(stdout)).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 apps; got %d", len(got))
	}
	wantOrder := []string{"Mail", "Safari", "Visual Studio Code"}
	for i, w := range wantOrder {
		if got[i].Name != w {
			t.Errorf("position %d: got %q want %q", i, got[i].Name, w)
		}
	}
}

func TestRunning_EmptyListing(t *testing.T) {
	t.Parallel()
	got, err := apps.New(fakeRunning("")).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("Running must return non-nil empty slice for callers that range")
	}
	if len(got) != 0 {
		t.Fatalf("expected zero apps; got %d", len(got))
	}
}

func TestRunning_HiddenFlagRoundTrip(t *testing.T) {
	t.Parallel()
	stdout := "Safari|1234|true\nMail|2345|false\n"
	got, err := apps.New(fakeRunning(stdout)).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	hiddenByName := map[string]bool{}
	for _, a := range got {
		hiddenByName[a.Name] = a.Hidden
	}
	if !hiddenByName["Safari"] {
		t.Errorf("Safari should be Hidden=true")
	}
	if hiddenByName["Mail"] {
		t.Errorf("Mail should be Hidden=false")
	}
}

func TestRunning_DropsMalformedLines(t *testing.T) {
	t.Parallel()
	// 1 valid, 1 wrong field count, 1 non-numeric pid, 1 valid.
	stdout := "Safari|1234|false\nNoPipes\nFoo|notanint|false\nMail|2345|true\n"
	got, err := apps.New(fakeRunning(stdout)).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 surviving apps; got %d (%+v)", len(got), got)
	}
}

func TestRunning_CaseInsensitiveSort(t *testing.T) {
	t.Parallel()
	stdout := "spotify|1|false\nFinder|2|false\nbrave|3|false\n"
	got, err := apps.New(fakeRunning(stdout)).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"brave", "Finder", "spotify"}
	for i, w := range wantOrder {
		if got[i].Name != w {
			t.Errorf("position %d: got %q want %q", i, got[i].Name, w)
		}
	}
}

func TestRunning_UnicodeName(t *testing.T) {
	t.Parallel()
	stdout := "Café\u00a0Notes|1234|false\n"
	got, err := apps.New(fakeRunning(stdout)).Running(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0].Name, "Café") {
		t.Fatalf("unicode lost: %+v", got)
	}
}

func TestRunning_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(runningScriptKey, "", errors.New("osascript: tcc denied"))
	if _, err := apps.New(f).Running(context.Background()); err == nil {
		t.Fatal("expected runner error to bubble")
	}
}

func TestQuit_IssuesExpectedScript(t *testing.T) {
	t.Parallel()
	wantArg := `tell application "Safari" to quit`
	f := runner.NewFake().On("osascript -e "+wantArg, "", nil)
	if err := apps.New(f).Quit(context.Background(), "Safari"); err != nil {
		t.Fatal(err)
	}
	calls := f.Calls()
	if len(calls) != 1 || len(calls[0].Args) != 2 || calls[0].Args[1] != wantArg {
		t.Fatalf("unexpected argv: %+v", calls)
	}
}

func TestQuit_EscapesQuotesAndBackslash(t *testing.T) {
	t.Parallel()
	// App name with both metacharacters; expect them escaped.
	name := `Foo "Bar" \\baz`
	wantArg := `tell application "Foo \"Bar\" \\\\baz" to quit`
	f := runner.NewFake().On("osascript -e "+wantArg, "", nil)
	if err := apps.New(f).Quit(context.Background(), name); err != nil {
		t.Fatal(err)
	}
}

func TestQuit_RunnerErrorBubbles(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e tell application "Safari" to quit`, "", errors.New("not running"))
	if err := apps.New(f).Quit(context.Background(), "Safari"); err == nil {
		t.Fatal("expected error from runner")
	}
}

func TestForceQuit_KillsByPID(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill -KILL 1234", "", nil)
	if err := apps.New(f).ForceQuit(context.Background(), 1234); err != nil {
		t.Fatal(err)
	}
}

func TestForceQuit_RejectsZeroOrNegative(t *testing.T) {
	t.Parallel()
	cases := []int{0, -1, -999}
	for _, pid := range cases {
		// Use a fake without any rules — if we mistakenly call
		// kill, the fake returns "no rule" which would be a
		// different (also-failing) error. But the function should
		// reject BEFORE the runner call.
		err := apps.New(runner.NewFake()).ForceQuit(context.Background(), pid)
		if err == nil {
			t.Errorf("pid=%d should be rejected", pid)
		}
		if !strings.Contains(err.Error(), "invalid pid") {
			t.Errorf("pid=%d expected 'invalid pid' message; got %q", pid, err)
		}
	}
}

func TestForceQuit_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill -KILL 1234", "", errors.New("no such process"))
	if err := apps.New(f).ForceQuit(context.Background(), 1234); err == nil {
		t.Fatal("expected error")
	}
}

func TestHide_IssuesExpectedScript(t *testing.T) {
	t.Parallel()
	wantArg := `tell application "System Events" to set visible of process "Safari" to false`
	f := runner.NewFake().On("osascript -e "+wantArg, "", nil)
	if err := apps.New(f).Hide(context.Background(), "Safari"); err != nil {
		t.Fatal(err)
	}
}

func TestHide_EscapesName(t *testing.T) {
	t.Parallel()
	name := `Foo"Bar`
	wantArg := `tell application "System Events" to set visible of process "Foo\"Bar" to false`
	f := runner.NewFake().On("osascript -e "+wantArg, "", nil)
	if err := apps.New(f).Hide(context.Background(), name); err != nil {
		t.Fatal(err)
	}
}

func TestHide_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e tell application "System Events" to set visible of process "Safari" to false`, "", errors.New("tcc"))
	if err := apps.New(f).Hide(context.Background(), "Safari"); err == nil {
		t.Fatal("expected error")
	}
}
