package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// brewDeps is the catalog of optional brew formulae that
// macontrol features depend on. [runDoctor] iterates this list
// to render the "are these installed?" report.
//
// Each entry pairs the binary name (used with [exec.LookPath]),
// a human-readable "why we need it" line shown next to the
// check mark, and a "how to install it" hint shown below.
//
// The list is the source of truth for the brew-deps section;
// adding a new optional dep means appending here AND ensuring
// the macontrol Homebrew formula declares it in its
// Depends_on directives so users on `brew install` get them
// automatically.
var brewDeps = []struct {
	// Bin is the binary name as it appears on $PATH.
	Bin string

	// Why is the human-readable feature label paired with the
	// check mark.
	Why string

	// Hint is the install instruction shown when the bin is
	// missing.
	Hint string
}{
	{"brightness", "💡 brightness control", "brew install brightness"},
	{"blueutil", "🔵 Bluetooth toggle/list/connect", "brew install blueutil"},
	{"terminal-notifier", "🔔 rich notifications (fallback: osascript)", "brew install terminal-notifier"},
	{"smctemp", "🌡 °C readings on Apple Silicon", "brew install smctemp"},
	{"imagesnap", "📸 webcam photos", "brew install imagesnap"},
}

// runDoctor is the implementation of `macontrol doctor`. Prints
// a human-readable self-check report covering version, runtime,
// macOS feature gates, brew dependencies, sudoers entry, and
// Keychain entries. Always exits 0 — the report itself is the
// signal.
//
// Behavior (in order):
//  1. Print version + commit + date (the same identity line
//     that `macontrol version` shows).
//  2. Print runtime GOOS/GOARCH. Warn when not darwin (only a
//     subset of checks make sense on Linux dev boxes).
//  3. Run [capability.Detect] and print the macOS version +
//     each gated feature flag. Subprocess failure prints a
//     warning and continues.
//  4. Iterate [brewDeps] and print ✓/✗ for each, plus the
//     install hint regardless of state (always-shown so the
//     user can copy-paste even when the dep is present and
//     they're checking the spelling).
//  5. Try `sudo -n pmset -g`. Success means the narrow
//     sudoers entry is installed; failure means it isn't.
//  6. Check the two Keychain entries via [checkKeychain].
//
// Used both interactively by users debugging "why doesn't /lock
// work?" and during initial install to confirm everything
// landed.
func runDoctor() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := runner.New()

	fmt.Printf("macontrol %s (%s, %s)\n", version, commit, date)
	fmt.Printf("runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS != "darwin" {
		fmt.Println("⚠ macontrol targets darwin/arm64. Only a subset of checks will run here.")
	}

	if rep, err := capability.Detect(ctx, r); err == nil {
		fmt.Println()
		fmt.Printf("macOS:            %s\n", rep.Version)
		fmt.Printf("networkQuality:   %v\n", rep.Features.NetworkQuality)
		fmt.Printf("shortcuts CLI:    %v\n", rep.Features.Shortcuts)
		fmt.Printf("wdutil info:      %v\n", rep.Features.WdutilInfo)
	} else {
		fmt.Printf("⚠ could not run sw_vers: %v\n", err)
	}

	fmt.Println()
	fmt.Println("brew deps (auto-installed when macontrol is installed via Homebrew):")
	for _, d := range brewDeps {
		mark := "✓"
		if _, err := exec.LookPath(d.Bin); err != nil {
			mark = "✗"
		}
		fmt.Printf("  %s %-18s %s\n     └ %s\n", mark, d.Bin, d.Why, d.Hint)
	}

	fmt.Println()
	fmt.Println("sudoers (pmset):")
	if _, err := r.Sudo(ctx, "pmset", "-g"); err == nil {
		fmt.Println("  ✓ `sudo -n pmset -g` works")
	} else {
		fmt.Println("  ✗ `sudo -n pmset -g` failed — install sudoers entry via `macontrol setup`")
	}

	fmt.Println()
	fmt.Println("keychain:")
	kc := keychain.New(r)
	account := currentUser()
	checkKeychain(ctx, kc, account, keychain.ServiceToken, "bot token")
	checkKeychain(ctx, kc, account, keychain.ServiceWhitelist, "whitelist")
}

// checkKeychain prints one Keychain-entry status line for
// [runDoctor] using the standard ✓ / ✗ / ⚠ convention.
//
// Routing rules (first match wins):
//  1. Get returned nil error → "✓ <label> present in Keychain
//     (<service>)".
//  2. Get returned [keychain.ErrNotFound] → "✗ <label> missing
//     from Keychain — run `macontrol setup` (or `macontrol
//     token set`/`whitelist add`)".
//  3. Get returned [keychain.ErrLocked] → "⚠ <label> present
//     but Keychain is locked — log in to unlock".
//  4. Any other error → "⚠ <label> lookup failed: <err>".
//
// Doesn't return anything; the side-effect is the printed line.
func checkKeychain(ctx context.Context, kc *keychain.Client, account, service, label string) {
	_, err := kc.Get(ctx, service, account)
	switch {
	case err == nil:
		fmt.Printf("  ✓ %-9s present in Keychain (%s)\n", label, service)
	case errors.Is(err, keychain.ErrNotFound):
		fmt.Printf("  ✗ %-9s missing from Keychain — run `macontrol setup` (or `macontrol token set`/`whitelist add`)\n", label)
	case errors.Is(err, keychain.ErrLocked):
		fmt.Printf("  ⚠ %-9s present but Keychain is locked — log in to unlock\n", label)
	default:
		fmt.Printf("  ⚠ %-9s lookup failed: %v\n", label, err)
	}
}
