package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/amiwrpremium/macontrol/internal/config"
)

// Build-time metadata. GoReleaser stamps these via -ldflags
// "-X main.version=…" so `macontrol version` and the daemon
// boot log show the actual build provenance.
//
//   - version is the semver tag (e.g. "0.6.0") or "dev" for
//     local builds.
//   - commit is the short git SHA (e.g. "a1b2c3d") or "none".
//   - date is the build timestamp in RFC3339 (e.g.
//     "2026-04-23T17:00:00Z") or "unknown".
//
// Local `go build`/`go run` doesn't stamp anything, hence the
// safe defaults.
var (
	// version is the semver tag stamped at build time.
	version = "dev"

	// commit is the short git SHA stamped at build time.
	commit = "none"

	// date is the RFC3339 build timestamp stamped at build
	// time.
	date = "unknown"
)

// main is the cmd/macontrol entry point. Dispatches to the
// matching subcommand based on os.Args[1], or to the daemon
// loop ([dispatchRun]) when no subcommand is given.
//
// Routing rules (os.Args[1] — first match wins):
//
//	Known subcommands:
//	 - "run"          → [dispatchRun] with the rest of the args.
//	 - "setup"        → [runSetup] (interactive first-run wizard).
//	 - "service"      → [runService] (LaunchAgent install/start/stop/logs).
//	 - "whitelist"    → [runWhitelist] (manage allowed Telegram user IDs).
//	 - "token"        → [runToken] (rotate / clear / re-grant the bot token).
//	 - "doctor"       → [runDoctor] (capability + brew + sudoers self-check).
//	 - "version" / "--version" / "-v" → print version + commit + date.
//	 - "help" / "--help" / "-h"       → [printHelp].
//
//	Else:
//	 - First arg starts with "-" → treat the whole tail as flags
//	   for `run` (so `macontrol --log-level=debug` works without
//	   the explicit "run" subcommand).
//	 - Anything else                → unknown subcommand error +
//	   help text + exit 2.
//
// Tests cannot reach this function directly because it
// terminates the process on most paths; coverage comes from
// the per-subcommand functions.
// subcommands maps a subcommand name to the function that
// handles it. Functions all take the remaining os.Args slice
// (after the subcommand word) so every entry shares a single
// signature.
var subcommands = map[string]func([]string){
	"run":       dispatchRun,
	"setup":     runSetup,
	"service":   runService,
	"whitelist": runWhitelist,
	"token":     runToken,
	"doctor":    func([]string) { runDoctor() },
	"version":   func([]string) { printVersion() },
	"--version": func([]string) { printVersion() },
	"-v":        func([]string) { printVersion() },
	"help":      func([]string) { printHelp() },
	"--help":    func([]string) { printHelp() },
	"-h":        func([]string) { printHelp() },
}

func main() {
	if len(os.Args) < 2 {
		dispatchRun(nil)
		return
	}
	if h, ok := subcommands[os.Args[1]]; ok {
		h(os.Args[2:])
		return
	}
	// Unknown subcommand — fall through to daemon if it looks like a flag.
	if os.Args[1][0] == '-' {
		dispatchRun(os.Args[1:])
		return
	}
	fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
	printHelp()
	os.Exit(2)
}

// printVersion writes the version / commit / date banner.
func printVersion() {
	fmt.Printf("macontrol %s (%s, %s)\n", version, commit, date)
}

// dispatchRun parses the daemon's flag set and hands control
// to [runDaemon]. The flag set is intentionally narrow: just
// log level and log file path. Other runtime knobs belong
// either in the Keychain (for secrets) or as new explicit
// flags (for non-secrets).
//
// Behavior:
//   - Constructs a [flag.FlagSet] with [flag.ExitOnError], so
//     parse failures terminate the process with a usage message.
//   - --log-level defaults to "info"; valid values are debug /
//     info / warn / error (validated downstream in
//     [runDaemon]).
//   - --log-file defaults to [config.DefaultLogPath]'s
//     "~/Library/Logs/macontrol/macontrol.log" — passing an
//     empty string logs to stderr instead.
//   - Calls [runDaemon] with the parsed values. Never returns
//     under normal operation; the daemon blocks on the bot's
//     long-poll loop.
func dispatchRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	logLevel := fs.String("log-level", "info", "log level: debug, info, warn, error")
	defaultLog, _ := config.DefaultLogPath()
	logFile := fs.String("log-file", defaultLog, "path to log file; empty string logs to stderr")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "Usage: macontrol run [flags]")
		fs.PrintDefaults()
	}
	// flag.ExitOnError handles parse failures.
	_ = fs.Parse(args)

	runDaemon(*logLevel, *logFile)
}

// printHelp writes the full subcommand reference to stdout.
// Triggered by "macontrol help", "--help", "-h", or as a
// follow-up to an unknown-subcommand error.
//
// The text is a single multi-line raw string literal kept in
// sync by hand with the [main] dispatch table and [dispatchRun]'s
// flag set. A future refactor could generate it from the table
// to remove the manual sync risk.
func printHelp() {
	fmt.Print(`macontrol — Telegram bot that controls your Mac.

Usage:
  macontrol [subcommand]

Subcommands:
  run [--log-level] [--log-file]
                      Run the daemon (default if no subcommand is given).
  setup               Interactive first-run wizard: token, user ids, sudoers, LaunchAgent.
  service install     Install LaunchAgent plist to ~/Library/LaunchAgents/ and bootstrap.
  service uninstall   Remove LaunchAgent plist.
  service start       launchctl bootstrap the service.
  service stop        launchctl bootout the service.
  service status      Print launchctl status.
  service logs        Tail ~/Library/Logs/macontrol/macontrol.log.
  whitelist list      Print whitelisted Telegram user IDs.
  whitelist add ID    Add a Telegram user ID to the whitelist.
  whitelist remove ID Remove a Telegram user ID.
  whitelist clear     Empty the whitelist (requires confirmation).
  token set           Interactively replace the bot token (validates via getMe).
  token clear         Remove the bot token from the Keychain.
  token reauth        Re-grant Keychain ACL after the binary moved.
  doctor              Print capability report, check brew deps, test sudoers.
  version             Print version + commit + build date.
  help                This message.

Run flags:
  --log-level LEVEL   debug | info (default) | warn | error
  --log-file PATH     log file path; default ~/Library/Logs/macontrol/macontrol.log
                      pass an empty string to log to stderr
`)
}
