// Package apps exposes a control surface for the Mac's running
// user-facing applications: list them, quit them gracefully,
// force-kill them, or hide their windows.
//
// Every read goes through `osascript` talking to System Events
// (the same path the System dashboard's Top processes uses), so
// the package needs Accessibility TCC to be granted to the
// macontrol binary. Failures bubble up via the runner; the
// handler layer surfaces them with the standard "TCC?" hint.
//
// Public surface:
//
//   - [App] — one running application snapshot.
//   - [Service] — the per-process control surface; one instance
//     on bot.Deps.Services.Apps.
//
// Reads are uncached; every [Service.Running] call shells out
// fresh. Writes are fire-and-forget; callers re-Get to observe
// the post-change state.
package apps

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// App is the read-side snapshot of one running user-facing
// application. Returned by [Service.Running].
//
// Lifecycle:
//   - Constructed by [parseAppsListing] for each line of
//     `osascript` output. Never cached across calls.
//
// Field roles:
//   - Name is the user-visible app name (e.g. "Safari", "Visual
//     Studio Code"). Used as the AppleScript handle for
//     [Service.Quit] and [Service.Hide].
//   - PID is the kernel process id. Used by [Service.ForceQuit]
//     so a SIGKILL targets the exact instance even when the user
//     has multiple copies of the same app open.
//   - Hidden mirrors the AppleScript "visible" property inverted:
//     true when the app's windows are hidden (Cmd-H state).
type App struct {
	// Name is the user-visible application name, e.g. "Safari".
	Name string

	// PID is the kernel process id of the running instance.
	PID int

	// Hidden is true when the app's windows are hidden (the
	// Cmd-H state). The list-render uses this for an icon hint;
	// no current code path acts on the value.
	Hidden bool
}

// Service is the macOS app-control surface. One instance per
// process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Apps.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     Each method shells out via the [runner.Runner], which is
//     itself concurrent-safe.
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// runningScript is the multi-line AppleScript [Service.Running]
// passes to `osascript -e`. Emits one `name|pid|hidden` line per
// user-facing process; trailing newline included.
//
// Pulled out as a const so the same string can be referenced as
// the [runner.Fake] key in tests.
const runningScript = "tell application \"System Events\"\n" +
	"set out to \"\"\n" +
	"repeat with p in (processes whose background only is false)\n" +
	"set out to out & (name of p) & \"|\" & (unix id of p) & \"|\" & ((not (visible of p)) as text) & linefeed\n" +
	"end repeat\n" +
	"return out\n" +
	"end tell"

// Running returns every user-facing application currently open,
// sorted alphabetically (case-insensitive) by [App.Name].
//
// Behavior:
//   - Shells out to a multi-line AppleScript that filters
//     `processes whose background only is false`. The filter
//     mirrors macOS's Force Quit menu and Activity Monitor's
//     "user interface" view.
//   - Output is one `name|pid|hidden` line per app. Parsed via
//     [parseAppsListing]; malformed lines are silently dropped
//     so a single weird name doesn't tank the whole listing.
//   - Sorts before return so callers get a stable order across
//     refreshes.
//   - Returns the runner error verbatim when osascript fails.
//     The most common cause is missing Accessibility TCC; the
//     handler layer surfaces that with the standard install /
//     grant hint.
func (s *Service) Running(ctx context.Context) ([]App, error) {
	out, err := s.r.Exec(ctx, "osascript", "-e", runningScript)
	if err != nil {
		return nil, err
	}
	apps := parseAppsListing(string(out))
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps, nil
}

// Quit asks the named application to quit gracefully via
// `osascript -e 'tell application "<name>" to quit'`.
//
// Behavior:
//   - The name is escaped via [escapeAppleScriptString] so quotes
//     and backslashes round-trip safely (rare in app names but
//     possible).
//   - AppleScript's `quit` is non-blocking and returns no error
//     when the app refuses (e.g. unsaved-document dialog). The
//     caller observes the unchanged state on the next [Running].
//   - Returns the runner error verbatim on osascript failure
//     (typically TCC denial or app-name-not-found).
func (s *Service) Quit(ctx context.Context, name string) error {
	script := fmt.Sprintf(`tell application "%s" to quit`, escapeAppleScriptString(name))
	_, err := s.r.Exec(ctx, "osascript", "-e", script)
	return err
}

// ForceQuit sends SIGKILL to pid via `kill -KILL <pid>`.
//
// Behavior:
//   - PID-based (not name-based) so a SIGKILL hits the exact
//     instance the user picked even when the app has multiple
//     copies open.
//   - Negative or zero pids are rejected with an error rather
//     than passed to `kill` (defends against accidentally
//     killing process groups).
//   - Returns the runner error verbatim on subprocess failure.
//     A non-existent pid returns "kill: <pid>: No such process".
func (s *Service) ForceQuit(ctx context.Context, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	_, err := s.r.Exec(ctx, "kill", "-KILL", strconv.Itoa(pid))
	return err
}

// Hide hides the named application's windows via
// `osascript -e 'tell application "System Events" to set visible of process "<name>" to false'`.
// Equivalent to the user pressing Cmd-H while the app is
// frontmost.
//
// Behavior:
//   - Escapes the name via [escapeAppleScriptString].
//   - Idempotent: hiding an already-hidden app is a no-op
//     (osascript returns no error).
//   - Returns the runner error verbatim on subprocess failure.
func (s *Service) Hide(ctx context.Context, name string) error {
	script := fmt.Sprintf(
		`tell application "System Events" to set visible of process "%s" to false`,
		escapeAppleScriptString(name),
	)
	_, err := s.r.Exec(ctx, "osascript", "-e", script)
	return err
}

// parseAppsListing decodes the `name|pid|hidden` lines from
// [runningScript]'s stdout into a slice of [App].
//
// Behavior:
//   - Splits stdout on newlines.
//   - Per line: splits on '|' with [strings.SplitN] capped at 3
//     so a name containing '|' would parse as a malformed pid
//     rather than corrupting the field count.
//   - Malformed lines (wrong field count, non-numeric pid,
//     non-boolean hidden) are silently dropped — a single bad
//     row doesn't tank the whole listing.
//   - Empty input returns an empty slice (not nil) so callers
//     can range without nil-check.
//
// Exported indirectly for the fuzz target.
func parseAppsListing(out string) []App {
	apps := make([]App, 0, 16)
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		hidden := strings.EqualFold(strings.TrimSpace(parts[2]), "true")
		apps = append(apps, App{
			Name:   strings.TrimSpace(parts[0]),
			PID:    pid,
			Hidden: hidden,
		})
	}
	return apps
}

// escapeAppleScriptString escapes the two characters that have
// special meaning inside a double-quoted AppleScript string:
// backslash (escape introducer) and double-quote (string
// terminator). Used by [Service.Quit] and [Service.Hide] to
// build safe inline scripts from user-controlled app names.
//
// Behavior:
//   - Backslash → `\\`. Must come first so the next replacement
//     doesn't double-escape its own backslashes.
//   - Double-quote → `\"`.
//   - Any other character passes through unchanged. AppleScript
//     accepts arbitrary UTF-8 inside quoted strings.
func escapeAppleScriptString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
	)
	return r.Replace(s)
}
