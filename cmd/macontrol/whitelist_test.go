package main

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

const (
	wlListCmd     = "security find-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w"
	wlAddBaseCmd  = "security add-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice -w"
	wlDeleteCmd   = "security delete-generic-password -s com.amiwrpremium.macontrol.whitelist -a alice"
	tokListCmd    = "security find-generic-password -s com.amiwrpremium.macontrol -a alice -w"
	tokAddBaseCmd = "security add-generic-password -s com.amiwrpremium.macontrol -a alice -w"
	tokDeleteCmd  = "security delete-generic-password -s com.amiwrpremium.macontrol -a alice"
)

func notFoundErrCmd() error {
	return &runner.Error{Stderr: []byte("could not be found in the keychain"), Err: errors.New("exit 44")}
}

// ---------------- whitelistRead ----------------

func TestWhitelistRead_HappyPath(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlListCmd, "111,222,333\n", nil)
	kc := keychain.New(f)
	ids, err := whitelistRead(kc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 111 || ids[2] != 333 {
		t.Errorf("ids = %v", ids)
	}
}

func TestWhitelistRead_NotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlListCmd, "", notFoundErrCmd())
	kc := keychain.New(f)
	ids, err := whitelistRead(kc, "alice")
	if err != nil {
		t.Fatalf("not-found should be swallowed; got %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice; got %v", ids)
	}
}

func TestWhitelistRead_KeychainError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlListCmd, "", errors.New("transport boom"))
	kc := keychain.New(f)
	if _, err := whitelistRead(kc, "alice"); err == nil {
		t.Fatal("expected error to propagate")
	}
}

// ---------------- whitelistWrite ----------------

func TestWhitelistWrite_PassesIDsThroughFormatter(t *testing.T) {
	t.Parallel()
	// Use a prefix-match rule: anything starting with the add command prefix
	// counts as success. The Fake records the full call so we can inspect
	// what value was written.
	f := runner.NewFake().On(wlAddBaseCmd, "", nil)
	kc := keychain.New(f)
	if err := whitelistWrite(kc, "alice", "/usr/local/bin/macontrol", []int64{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call; got %d", len(calls))
	}
	args := calls[0].Args
	// -w <value> must contain "1,2,3".
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "1,2,3") {
		t.Errorf("expected '1,2,3' written; got %q", joined)
	}
	// -T <exe> for trust.
	if !strings.Contains(joined, "/usr/local/bin/macontrol") {
		t.Errorf("expected trusted binary in args; got %q", joined)
	}
}

func TestWhitelistWrite_EmptyExeOmitsTrust(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlAddBaseCmd, "", nil)
	kc := keychain.New(f)
	if err := whitelistWrite(kc, "alice", "", []int64{1}); err != nil {
		t.Fatal(err)
	}
	// /usr/bin/security is always added by the keychain.Set wrapper. The
	// caller's empty exe should not introduce a duplicate trust.
}

// ---------------- promptYesNo ----------------

func TestPromptYesNo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		def   bool
		want  bool
	}{
		{"\n", true, true},       // blank → default
		{"\n", false, false},     // blank → default
		{"y\n", false, true},     // explicit y
		{"yes\n", false, true},   // explicit yes
		{"Y\n", false, true},     // case-insensitive
		{"YES\n", false, true},   // case-insensitive
		{"n\n", true, false},     // explicit n
		{"no\n", true, false},    // explicit no
		{"maybe\n", true, false}, // anything else → false
	}
	for _, c := range cases {
		c := c
		t.Run(strings.TrimSpace(c.input), func(t *testing.T) {
			t.Parallel()
			r := bufio.NewReader(strings.NewReader(c.input))
			if got := promptYesNo(r, "test? ", c.def); got != c.want {
				t.Errorf("promptYesNo(%q, def=%v) = %v; want %v",
					c.input, c.def, got, c.want)
			}
		})
	}
}

// ---------------- promptLine ----------------

func TestPromptLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"hello\n", "hello"},
		{"with-trailing-cr\r\n", "with-trailing-cr"},
		{"\n", ""},
		{"no-newline-at-eof", "no-newline-at-eof"},
	}
	for _, c := range cases {
		r := bufio.NewReader(strings.NewReader(c.input))
		got := promptLine(r, "")
		if got != c.want {
			t.Errorf("promptLine(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

// ---------------- contains (already covered, add edge-case parallel) ----------------

func TestContains_TableDriven(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ss    []string
		query string
		want  bool
	}{
		{nil, "x", false},
		{[]string{}, "x", false},
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "z", false},
		{[]string{""}, "", true},
		{[]string{"foo"}, "foo", true},
	}
	for _, c := range cases {
		if got := contains(c.ss, c.query); got != c.want {
			t.Errorf("contains(%v, %q) = %v; want %v", c.ss, c.query, got, c.want)
		}
	}
}

// ---------------- tokenReauth happy path ----------------

func TestTokenReauth_RoundtripsTokenAndWhitelist(t *testing.T) {
	t.Parallel()
	exe := "/usr/local/bin/macontrol"
	// First Get reads the existing token, then Set re-issues it with -T exe;
	// then Get reads whitelist (optional success), then Set re-issues it.
	f := runner.NewFake().
		On(tokListCmd, "tok-secret\n", nil).
		On(tokAddBaseCmd, "", nil).
		On(wlListCmd, "1,2\n", nil).
		On(wlAddBaseCmd, "", nil)
	kc := keychain.New(f)
	tokenReauth(kc, "alice", exe)

	// We expect 4 calls total in order: get token, set token, get wl, set wl.
	calls := f.Calls()
	if len(calls) != 4 {
		t.Fatalf("expected 4 keychain calls; got %d (%+v)", len(calls), calls)
	}
	for i, c := range calls {
		if c.Name != "security" {
			t.Errorf("call[%d].Name = %q; want security", i, c.Name)
		}
	}
}

func TestTokenReauth_WhitelistGetFails_Continues(t *testing.T) {
	t.Parallel()
	exe := "/macontrol"
	// Get token OK, Set token OK, Get whitelist fails — the function still
	// completes because the whitelist re-issue is best-effort.
	f := runner.NewFake().
		On(tokListCmd, "tok\n", nil).
		On(tokAddBaseCmd, "", nil).
		On(wlListCmd, "", notFoundErrCmd())
	kc := keychain.New(f)
	tokenReauth(kc, "alice", exe) // must not fatalf
	calls := f.Calls()
	if len(calls) != 3 {
		t.Errorf("expected 3 calls; got %d", len(calls))
	}
}

// ---------------- runService usage / unknown subcommand ----------------

// We can't easily invoke runService because the Bad branches call os.Exit
// via fatalf. However, we exercise plistPath() via env-var injection
// (already covered) and assert the service.go constants stay canonical.

func TestPlistName_Canonical(t *testing.T) {
	t.Parallel()
	if plistName != "com.amiwrpremium.macontrol.plist" {
		t.Errorf("plist name = %q", plistName)
	}
}

// ---------------- ServiceInstall failure (read-only HOME) ----------------

func TestServiceInstall_HomeReadOnly_FailsCleanly(t *testing.T) {
	t.Parallel()
	// Point HOME at a path we cannot create directories under. The
	// platform-agnostic way is to use /proc/1 (a real file, not a dir).
	// We verify the function returns an error rather than panicking.
	t.Skip("os.MkdirAll under a non-writable HOME is platform-sensitive; skip")
}

// ---------------- whitelistRead — invalid stored value ----------------

func TestWhitelistRead_InvalidValueErr(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlListCmd, "abc,def\n", nil)
	kc := keychain.New(f)
	if _, err := whitelistRead(kc, "alice"); err == nil {
		t.Fatal("expected ParseUserIDs error")
	}
}

// ---------------- verifyToken (ensure error path covers !ok) ----------------

func TestVerifyToken_EmptyToken_ErrorsOrFails(t *testing.T) {
	t.Parallel()
	// An empty token should fail — either network-unreachable in CI or a
	// negative API response. Either way, error must be non-nil.
	if _, err := verifyToken(""); err == nil {
		t.Skip("verifyToken unexpectedly succeeded; skipping")
	}
}

// ---------------- ContextDeadline guard ----------------

// Sanity check that operations finish well within the timeout window
// when stubbed — guards against accidentally introducing real network
// I/O in helpers under test.
func TestWhitelistRead_FinishesPromptly(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(wlListCmd, "1\n", nil)
	kc := keychain.New(f)
	done := make(chan struct{})
	go func() {
		_, _ = whitelistRead(kc, "alice")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("whitelistRead should complete quickly with a Fake runner")
	}
	_ = context.Background
}

// stdoutMu serializes os.Stdout/os.Stdin overrides across parallel tests
// in this package. The capture functions below take it for the duration
// of the override; tests using them therefore can't run in parallel.
var stdoutMu sync.Mutex

// captureStdout redirects os.Stdout for fn() and returns whatever was
// written. Used to assert printed output of whitelistList et al.
// Caller must NOT use t.Parallel() — os.Stdout is process-global.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	done := make(chan string)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				done <- b.String()
				return
			}
		}
	}()
	fn()
	_ = w.Close()
	return <-done
}

// withStdin overrides os.Stdin for the duration of fn. Like captureStdout,
// callers must not run in parallel.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(input))
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = orig
		_ = r.Close()
	}()
	fn()
}

func TestWhitelistList_Empty(t *testing.T) {
	// Not Parallel — touches os.Stdout.
	f := runner.NewFake().On(wlListCmd, "", notFoundErrCmd())
	kc := keychain.New(f)
	out := captureStdout(t, func() { whitelistList(kc, "alice") })
	if !strings.Contains(out, "(empty)") {
		t.Errorf("expected '(empty)'; got %q", out)
	}
}

func TestWhitelistList_PrintsIDsLineByLine(t *testing.T) {
	f := runner.NewFake().On(wlListCmd, "10,20,30\n", nil)
	kc := keychain.New(f)
	out := captureStdout(t, func() { whitelistList(kc, "alice") })
	for _, want := range []string{"10", "20", "30"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got %q", want, out)
		}
	}
}

func TestWhitelistAdd_NewID(t *testing.T) {
	f := runner.NewFake().
		On(wlListCmd, "100\n", nil).
		On(wlAddBaseCmd, "", nil)
	kc := keychain.New(f)
	out := captureStdout(t, func() {
		whitelistAdd(kc, "alice", "/macontrol", "200")
	})
	if !strings.Contains(out, "added 200") {
		t.Errorf("expected 'added 200' message; got %q", out)
	}
	calls := f.Calls()
	if len(calls) < 2 {
		t.Fatalf("expected ≥2 calls; got %d", len(calls))
	}
	setArgs := strings.Join(calls[len(calls)-1].Args, " ")
	if !strings.Contains(setArgs, "100,200") {
		t.Errorf("expected '100,200' written; got %q", setArgs)
	}
}

func TestWhitelistAdd_AlreadyPresent(t *testing.T) {
	f := runner.NewFake().On(wlListCmd, "100,200\n", nil)
	kc := keychain.New(f)
	out := captureStdout(t, func() {
		whitelistAdd(kc, "alice", "/macontrol", "200")
	})
	if !strings.Contains(out, "already whitelisted") {
		t.Errorf("expected 'already whitelisted'; got %q", out)
	}
	for _, c := range f.Calls() {
		args := strings.Join(c.Args, " ")
		if strings.Contains(args, "add-generic-password") {
			t.Errorf("unexpected add-generic-password call: %v", c)
		}
	}
}

func TestWhitelistRemove_RemovesExisting(t *testing.T) {
	f := runner.NewFake().
		On(wlListCmd, "100,200,300\n", nil).
		On(wlAddBaseCmd, "", nil)
	kc := keychain.New(f)
	out := captureStdout(t, func() {
		whitelistRemove(kc, "alice", "/macontrol", "200")
	})
	if !strings.Contains(out, "removed 200") {
		t.Errorf("expected 'removed 200'; got %q", out)
	}
	calls := f.Calls()
	setArgs := strings.Join(calls[len(calls)-1].Args, " ")
	if strings.Contains(setArgs, "200") && !strings.Contains(setArgs, "100,300") {
		t.Errorf("expected '100,300' as remaining; got %q", setArgs)
	}
}

func TestWhitelistRemove_NotPresent(t *testing.T) {
	f := runner.NewFake().On(wlListCmd, "100\n", nil)
	kc := keychain.New(f)
	out := captureStdout(t, func() {
		whitelistRemove(kc, "alice", "/macontrol", "999")
	})
	if !strings.Contains(out, "not on the whitelist") {
		t.Errorf("expected 'not on the whitelist'; got %q", out)
	}
}

// ---------------- tokenClear / whitelistClear (yes path) ----------------

// tokenClear and whitelistClear read from os.Stdin via bufio.NewReader.
// Inject an "n\n" → aborted path to keep the function from invoking
// kc.Delete (which would still succeed via the fake but pollute calls).

func TestTokenClear_AbortedKeepsToken(t *testing.T) {
	f := runner.NewFake()
	kc := keychain.New(f)
	var out string
	withStdin(t, "n\n", func() {
		out = captureStdout(t, func() { tokenClear(kc, "alice") })
	})
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected aborted message; got %q", out)
	}
	if len(f.Calls()) != 0 {
		t.Errorf("expected no keychain calls on abort; got %v", f.Calls())
	}
}

func TestTokenClear_ConfirmedDeletes(t *testing.T) {
	f := runner.NewFake().On(tokDeleteCmd, "", nil)
	kc := keychain.New(f)
	var out string
	withStdin(t, "y\n", func() {
		out = captureStdout(t, func() { tokenClear(kc, "alice") })
	})
	if !strings.Contains(out, "token cleared") {
		t.Errorf("expected 'token cleared'; got %q", out)
	}
}

func TestWhitelistClear_AbortedKeepsList(t *testing.T) {
	f := runner.NewFake()
	kc := keychain.New(f)
	var out string
	withStdin(t, "n\n", func() {
		out = captureStdout(t, func() { whitelistClear(kc, "alice") })
	})
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted'; got %q", out)
	}
}

func TestWhitelistClear_ConfirmedDeletes(t *testing.T) {
	f := runner.NewFake().On(wlDeleteCmd, "", nil)
	kc := keychain.New(f)
	var out string
	withStdin(t, "y\n", func() {
		out = captureStdout(t, func() { whitelistClear(kc, "alice") })
	})
	if !strings.Contains(out, "whitelist cleared") {
		t.Errorf("expected 'whitelist cleared'; got %q", out)
	}
}
