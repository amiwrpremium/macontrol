package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// runSetup is the interactive first-run wizard for the
// `macontrol setup` subcommand. Walks the user through token
// + whitelist + LaunchAgent + sudoers configuration in one
// terminal session.
//
// Lifecycle (in order):
//  1. Check for "--reconfigure" in args (lets the wizard
//     overwrite existing Keychain entries).
//  2. Refuse to proceed if a token is already stored AND
//     --reconfigure was NOT passed — defends against
//     accidental overwrite.
//  3. Prompt for the bot token (hidden input via
//     [promptHidden] — uses syscall TTY ioctl to suppress
//     echo).
//  4. Prompt for the primary user ID (visible input).
//  5. Prompt for additional user IDs (comma-separated).
//  6. Verify the token via [verifyToken] (calls Telegram's
//     getMe endpoint). Aborts on failure.
//  7. Store both secrets in the macOS Keychain via
//     [keychain.Client.Set], trusting the macontrol binary
//     so the daemon can read silently after the first prompt.
//  8. Optionally install the LaunchAgent via
//     [serviceInstall] (Y default).
//  9. Optionally install the narrow sudoers entry via
//     [installSudoersFile] (N default).
//  10. Print TCC permission reminders that the wizard CAN'T
//     automate (Apple's privacy boundary).
//  11. Optionally start the daemon via [serviceStart].
//
// Exits non-zero via [fatalf] on any unrecoverable error;
// does not return cleanly under normal operation.
func runSetup(args []string) {
	reconfigure := contains(args, "--reconfigure")
	fmt.Println("macontrol first-run setup. Press Ctrl-C to abort.")
	fmt.Println()

	kc := keychain.New(runner.New())
	account := currentUser()
	exe, _ := os.Executable()

	// Refuse to overwrite without --reconfigure.
	if !reconfigure {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := kc.Get(ctx, keychain.ServiceToken, account)
		cancel()
		if err == nil {
			fmt.Println("⚠ a token is already stored in the Keychain")
			fmt.Println("   run `macontrol setup --reconfigure` to overwrite.")
			return
		}
	}

	in := bufio.NewReader(os.Stdin)

	token := promptHidden("▸ Telegram bot token (from @BotFather): ")
	if token == "" {
		fatalf("token is required")
	}
	primary := promptLine(in, "▸ Your Telegram user ID (from @userinfobot): ")
	primary = strings.TrimSpace(primary)
	if _, err := strconv.ParseInt(primary, 10, 64); err != nil {
		fatalf("user id must be an integer, got %q", primary)
	}
	extra := promptLine(in, "▸ Additional user IDs to allow, comma-separated (blank = none): ")
	ids := strings.TrimSpace(primary)
	if extra = strings.TrimSpace(extra); extra != "" {
		ids = ids + "," + extra
	}

	fmt.Print("▸ Verifying token…  ")
	botUser, err := verifyToken(token)
	if err != nil {
		fmt.Println("✗")
		fatalf("token verification failed: %v", err)
	}
	fmt.Printf("✓ bot @%s\n", botUser)

	// Store both secrets in the Keychain. Trust the macontrol binary so
	// the daemon can read silently after the first prompt.
	storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer storeCancel()
	trusted := []string{}
	if exe != "" {
		trusted = append(trusted, exe)
	}
	if err := kc.Set(storeCtx, keychain.ServiceToken, account, token, trusted...); err != nil {
		fatalf("storing token in Keychain: %v", err)
	}
	if err := kc.Set(storeCtx, keychain.ServiceWhitelist, account, ids, trusted...); err != nil {
		fatalf("storing whitelist in Keychain: %v", err)
	}
	fmt.Println("▸ Stored token + whitelist in Keychain  ✓")

	installAgent := promptYesNo(in, "▸ Install LaunchAgent so macontrol starts at login? [Y/n] ", true)
	if installAgent {
		if err := serviceInstall(); err != nil {
			fmt.Printf("⚠ could not install LaunchAgent: %v\n", err)
		} else {
			fmt.Println("▸ LaunchAgent installed  ✓")
		}
	}

	installSudoers := promptYesNo(in, "▸ Install narrow sudoers entry (shutdown/pmset/wdutil/powermetrics/systemsetup)? [y/N] ", false)
	if installSudoers {
		if err := installSudoersFile(); err != nil {
			fmt.Printf("⚠ could not install sudoers entry: %v\n", err)
			fmt.Println("  You can install it later by copying sudoers.d/macontrol.sample to /etc/sudoers.d/macontrol via `sudo visudo -f /etc/sudoers.d/macontrol`.")
		} else {
			fmt.Println("▸ /etc/sudoers.d/macontrol written  ✓")
		}
	}

	fmt.Println()
	fmt.Println("TCC permissions to grant (System Settings → Privacy & Security):")
	fmt.Println("  • Screen Recording  — /screenshot, /record")
	fmt.Println("  • Accessibility     — app listing, fallback brightness")
	fmt.Println("  • Camera            — /photo")
	fmt.Println()

	if installAgent && promptYesNo(in, "▸ Start the daemon now? [Y/n] ", true) {
		if err := serviceStart(); err != nil {
			fmt.Printf("⚠ start failed: %v\n", err)
		} else {
			fmt.Println("  daemon started.")
		}
	}
	fmt.Printf("\nDone. Send /start to @%s.\n", botUser)
}

// promptLine reads one line of (visible) input from the
// supplied bufio.Reader and returns it with trailing CR/LF
// stripped.
//
// Behavior:
//   - Prints label to stdout (no automatic newline).
//   - Reads up to '\n' or EOF; returns the captured string
//     minus trailing "\r\n" or "\n".
//   - Read errors are silently swallowed — callers see an
//     empty string, which most prompt validation rejects.
func promptLine(in *bufio.Reader, label string) string {
	fmt.Print(label)
	s, _ := in.ReadString('\n')
	return strings.TrimRight(s, "\r\n")
}

// promptHidden reads one line of HIDDEN input via
// [term.ReadPassword] (TTY ioctl that suppresses echo).
// Used for the bot token so it doesn't appear on screen and
// doesn't end up in shell history if the user pastes from
// another terminal.
//
// Behavior:
//   - Prints label, reads with echo off, prints a newline
//     after.
//   - Returns the trimmed input.
//   - Calls [fatalf] on read errors (typically: stdin isn't
//     a TTY — the user piped input which can't have echo
//     suppressed).
func promptHidden(label string) string {
	fmt.Print(label)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fatalf("could not read hidden input: %v", err)
	}
	return strings.TrimSpace(string(b))
}

// promptYesNo prompts a Y/N question with a default value when
// the user just presses Enter.
//
// Behavior:
//   - Reads via [promptLine].
//   - Empty input → returns def.
//   - "y" or "yes" (case-insensitive) → returns true.
//   - Anything else → returns false.
//
// The wizard renders the def in the prompt label as "[Y/n]"
// or "[y/N]" — this function trusts the caller to keep that
// in sync; there's no automatic capitalisation hint.
func promptYesNo(in *bufio.Reader, label string, def bool) bool {
	s := promptLine(in, label)
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def
	}
	return s == "y" || s == "yes"
}

// verifyToken calls Telegram's `/bot<token>/getMe` endpoint
// to confirm the token is valid and returns the bot's
// @-username on success.
//
// Behavior:
//   - 10-second context timeout — the wizard shouldn't hang
//     on a slow network.
//   - URL-escapes the token (Telegram tokens contain ':' and
//     '_' which are safe in path segments but the escape is
//     defensive).
//   - Decodes the response into a typed struct; checks the
//     `ok` field.
//   - Returns the captured Username on success.
//   - Returns "telegram API: <description>" on `ok=false`
//     (typical: "Unauthorized" for a wrong token).
//   - Returns the http or json error verbatim on transport /
//     parse failure.
func verifyToken(token string) (string, error) {
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, "GET",
		"https://api.telegram.org/bot"+url.PathEscape(token)+"/getMe", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if !parsed.OK {
		return "", fmt.Errorf("telegram API: %s", parsed.Description)
	}
	return parsed.Result.Username, nil
}

// contains reports whether ss includes needle. Linear scan.
// Used by [runSetup] to detect "--reconfigure" in args without
// reaching for [flag.FlagSet] for a single switch.
func contains(ss []string, needle string) bool {
	for _, s := range ss {
		if s == needle {
			return true
		}
	}
	return false
}

// fatalf prints a "macontrol setup: <message>" line to stderr
// and exits the process with status 1. Used for unrecoverable
// wizard errors where there's nothing the user can do beyond
// re-running with corrected input.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "macontrol setup: "+format+"\n", args...)
	os.Exit(1)
}

// installSudoersFile writes the narrow sudoers entry to
// /etc/sudoers.d/macontrol via `sudo install` after validating
// it with `sudo visudo -cf`.
//
// Behavior (in order):
//  1. Renders the entry body via [sudoersBody] (which
//     templates in the current Unix user).
//  2. Writes it to a fresh tempfile via [os.CreateTemp]; the
//     defer cleans up regardless of outcome.
//  3. Runs `sudo visudo -cf <tmp>` to syntax-check.
//     visudo prompts for the user's sudo password
//     interactively (stdin/out/err are wired to the wizard's
//     TTY). Returns "visudo check failed: <err>" on syntax
//     error.
//  4. Runs `sudo install -m 0440 -o root -g wheel <tmp>
//     /etc/sudoers.d/macontrol` to atomically install the
//     file with the correct mode + ownership.
//
// Returns the install command's error, or nil on success.
//
// Why install instead of cp: install is atomic (write-temp +
// rename), preserves the explicit mode/owner flags, and is
// what the homebrew formula uses too — keeps the install path
// consistent regardless of whether the user came from the
// wizard or from `brew postinstall`.
func installSudoersFile() error {
	content := sudoersBody()
	tmp, err := os.CreateTemp("", "macontrol-sudoers-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	_ = tmp.Close()

	// Validate locally first.
	check := exec.Command("sudo", "visudo", "-cf", tmp.Name())
	check.Stdin = os.Stdin
	check.Stdout = os.Stdout
	check.Stderr = os.Stderr
	if err := check.Run(); err != nil {
		return fmt.Errorf("visudo check failed: %w", err)
	}
	// Install.
	install := exec.Command("sudo", "install", "-m", "0440", "-o", "root", "-g", "wheel",
		tmp.Name(), "/etc/sudoers.d/macontrol")
	install.Stdin = os.Stdin
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	return install.Run()
}
