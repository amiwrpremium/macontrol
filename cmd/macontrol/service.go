package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// LaunchAgent identity used by every `macontrol service`
// subcommand. Both constants are the source of truth — the
// plist body itself (see [plistBody] in assets.go) reads
// plistLabel as its <key>Label</key> value, and plistName is
// the file basename under ~/Library/LaunchAgents/.
//
// Changing either breaks existing installs (the bootstrap path
// uses Label and the path uses Name); a rename needs a
// migration story (uninstall old, install new).
const (
	// plistLabel is the LaunchAgent's <Label>, used by
	// `launchctl bootstrap` / `bootout` to identify the
	// service.
	plistLabel = "com.amiwrpremium.macontrol"

	// plistName is the .plist file basename written under
	// ~/Library/LaunchAgents/.
	plistName = "com.amiwrpremium.macontrol.plist"
)

// runService is the implementation of `macontrol service
// {install|uninstall|start|stop|status|logs}`. Dispatches the
// args[0] subcommand to the matching helper.
//
// Routing rules (args[0] — first match wins):
//  1. "install"   → write the plist + bootstrap. See
//     [serviceInstall].
//  2. "uninstall" → bootout + delete the plist. See
//     [serviceUninstall].
//  3. "start"     → launchctl bootstrap. See [serviceStart].
//  4. "stop"      → launchctl bootout. See [serviceStop].
//  5. "status"    → launchctl print. See [serviceStatus].
//  6. "logs"      → tail -f the daemon log. See [serviceLogs].
//
// Empty args prints usage + exits 2; unknown subcommands route
// to [fatalf] (exits 1 with the macontrol-setup-prefixed
// message).
func runService(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: macontrol service {install|uninstall|start|stop|status|logs}")
		os.Exit(2)
	}
	switch args[0] {
	case "install":
		if err := serviceInstall(); err != nil {
			fatalf("service install: %v", err)
		}
		fmt.Println("installed.")
	case "uninstall":
		if err := serviceUninstall(); err != nil {
			fatalf("service uninstall: %v", err)
		}
		fmt.Println("uninstalled.")
	case "start":
		if err := serviceStart(); err != nil {
			fatalf("service start: %v", err)
		}
	case "stop":
		if err := serviceStop(); err != nil {
			fatalf("service stop: %v", err)
		}
	case "status":
		serviceStatus()
	case "logs":
		serviceLogs()
	default:
		fatalf("unknown service subcommand: %s", args[0])
	}
}

// plistPath returns the absolute path where the LaunchAgent
// plist is (or will be) installed —
// $HOME/Library/LaunchAgents/<plistName>.
//
// The home-dir lookup ignores its error; on a host without a
// resolvable home (extremely rare on macOS) the function
// returns "Library/LaunchAgents/…" as a relative path, which
// then fails downstream when launchctl tries to read it. The
// failure surface is acceptable because the wizard already
// caught the no-home case earlier.
func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

// serviceInstall writes the LaunchAgent plist to
// [plistPath] and immediately calls [serviceStart] to
// bootstrap it.
//
// Behavior:
//  1. Creates ~/Library/LaunchAgents/ with mode 0o750 if it
//     doesn't exist.
//  2. Resolves the macontrol binary path via [os.Executable]
//     so the plist's ProgramArguments references the actual
//     daemon binary, not whatever the user later moves it to.
//  3. Renders the plist body via [plistBody] (defined in
//     assets.go) with the binary path + the log directory.
//  4. Writes the plist with mode 0o600 (per launchd's
//     expectation that user LaunchAgents not be world-
//     readable).
//  5. Calls [serviceStart] to bootstrap immediately.
//
// Returns the first error from any step.
func serviceInstall() error {
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	body := plistBody(binary, filepath.Join(home, "Library", "Logs", "macontrol"))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	return serviceStart()
}

// serviceUninstall reverses [serviceInstall]: best-effort
// stops the running daemon via [serviceStop], then removes the
// plist file.
//
// Behavior:
//   - Stop errors are intentionally swallowed — uninstall must
//     succeed even when the agent isn't running.
//   - Returns the file-removal error verbatim. The most common
//     case is "no such file" when uninstall is run on an
//     already-uninstalled host; that's a noisy success but
//     not strictly broken.
func serviceUninstall() error {
	_ = serviceStop()
	path := plistPath()
	return os.Remove(path)
}

// serviceStart bootstraps the macontrol LaunchAgent into the
// current GUI session via `launchctl bootstrap gui/<uid>
// <plist-path>`.
//
// Behavior:
//   - Uses [os.Getuid] for the GUI session id (gui/<uid>),
//     which is the modern launchctl idiom for per-user agents
//     since macOS 10.10.
//   - Wires stdout/stderr to the wizard's TTY so the user
//     sees launchctl's own diagnostics.
//   - Returns the launchctl exit error verbatim. Common
//     failure: "Bootstrap failed: 5: Input/output error" when
//     the plist is malformed or already loaded.
func serviceStart() error {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "bootstrap", "gui/"+uid, plistPath())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// serviceStop boots the macontrol LaunchAgent OUT of the
// current GUI session via `launchctl bootout gui/<uid>/
// <label>`.
//
// Behavior:
//   - Same `gui/<uid>` idiom as [serviceStart].
//   - Wires stdout/stderr to the wizard's TTY.
//   - Returns the launchctl exit error verbatim. "Boot-out
//     failed: 5" when the agent isn't loaded — typically a
//     no-op success from the user's POV.
func serviceStop() error {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "bootout", "gui/"+uid+"/"+plistLabel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// serviceStatus prints the launchctl status block for the
// macontrol agent via `launchctl print gui/<uid>/<label>`.
//
// Behavior:
//   - Wires stdout/stderr to the wizard's TTY so the user
//     sees launchctl's verbose status block (PID, last exit
//     reason, throttle history, etc.).
//   - Errors are silently ignored — `launchctl print` returns
//     non-zero when the agent isn't loaded, which we treat as
//     a valid "no status" rather than a failure.
//
// Returns nothing; the side-effect is the printed block.
func serviceStatus() {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "print", "gui/"+uid+"/"+plistLabel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// serviceLogs runs `tail -n 200 -f` on the daemon's log file
// at ~/Library/Logs/macontrol/macontrol.log. Blocks until
// the user Ctrl-C's tail.
//
// Behavior:
//   - Wires stdout/stderr to the wizard's TTY so the user
//     sees the live log stream.
//   - Errors are silently ignored — the typical failure is
//     "no such file" before the daemon has run for the first
//     time, and the message tail prints is informative
//     enough.
//
// Returns nothing; blocks for the duration of the tail.
func serviceLogs() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "Logs", "macontrol", "macontrol.log")
	cmd := exec.Command("tail", "-n", "200", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
