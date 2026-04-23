package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/amiwrpremium/macontrol/internal/config"
)

// version metadata — populated at link time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		dispatchRun(nil)
		return
	}
	switch os.Args[1] {
	case "run":
		dispatchRun(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "service":
		runService(os.Args[2:])
	case "whitelist":
		runWhitelist(os.Args[2:])
	case "token":
		runToken(os.Args[2:])
	case "doctor":
		runDoctor()
	case "version", "--version", "-v":
		fmt.Printf("macontrol %s (%s, %s)\n", version, commit, date)
	case "help", "--help", "-h":
		printHelp()
	default:
		// Unknown subcommand — fall through to daemon if it looks like a flag.
		if os.Args[1][0] == '-' {
			dispatchRun(os.Args[1:])
			return
		}
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		printHelp()
		os.Exit(2)
	}
}

// dispatchRun parses the flag set for `macontrol run` and hands control to
// the daemon loop. Flags are kept narrow: just log level and log file. Any
// other runtime knobs should be expressed as Keychain entries (secrets) or
// new flags (non-secrets).
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
