package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// runToken is the implementation of `macontrol token
// {set|clear|reauth}`. Manages the bot token stored in the
// [keychain.ServiceToken] entry.
//
// Routing rules (args[0] — first match wins):
//  1. "set"    → [tokenSet] (interactively replace the token,
//     verifying via getMe first).
//  2. "clear"  → [tokenClear] (asks for confirmation; daemon
//     refuses to start afterwards).
//  3. "reauth" → [tokenReauth] (re-issue the Keychain entry
//     with a fresh trusted-binary ACL — useful when the
//     macontrol binary moved between locations).
//
// Empty args → usage line + exit 2. Unknown subcommand →
// [fatalf] (exits 1).
func runToken(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: macontrol token {set|clear|reauth}")
		os.Exit(2)
	}
	kc := keychain.New(runner.New())
	account := currentUser()
	exe, _ := os.Executable()

	switch args[0] {
	case "set":
		tokenSet(kc, account, exe)
	case "clear":
		tokenClear(kc, account)
	case "reauth":
		tokenReauth(kc, account, exe)
	default:
		fatalf("unknown token subcommand: %s", args[0])
	}
}

// tokenSet interactively replaces the stored bot token.
//
// Behavior:
//  1. Read the new token via [promptHidden] — never echoes,
//     never lands in shell history.
//  2. Reject empty input via [fatalf].
//  3. Verify via [verifyToken] (calls Telegram getMe). On
//     failure prints "✗" + error and exits.
//  4. On success, store via [keychain.Client.Set] with the
//     macontrol binary in the trusted-binary list.
//  5. Print success line + the standard "restart the daemon"
//     reminder.
//
// Used both as part of the initial wizard's flow and
// independently when rotating a leaked or expired token.
func tokenSet(kc *keychain.Client, account, exe string) {
	tok := promptHidden("▸ Telegram bot token: ")
	if tok == "" {
		fatalf("token is required")
	}

	fmt.Print("▸ Verifying…  ")
	botUser, err := verifyToken(tok)
	if err != nil {
		fmt.Println("✗")
		fatalf("token verification failed: %v", err)
	}
	fmt.Printf("✓ bot @%s\n", botUser)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	trusted := []string{}
	if exe != "" {
		trusted = append(trusted, exe)
	}
	if err := kc.Set(ctx, keychain.ServiceToken, account, tok, trusted...); err != nil {
		fatalf("storing token in Keychain: %v", err)
	}
	fmt.Println("✓ token stored in Keychain")
	fmt.Println("restart the daemon for it to take effect: brew services restart macontrol")
}

// tokenClear deletes the [keychain.ServiceToken] entry after
// a y/N prompt. The daemon refuses to start without a token,
// so this is effectively a kill-switch.
//
// Behavior:
//  1. Read y/N via [promptYesNo] with default N.
//  2. Call [keychain.Client.Delete]; treats ErrNotFound as
//     success (already gone).
//  3. Print success line.
//
// Used when retiring a bot, or as the first step of a manual
// reset before re-running [runSetup].
func tokenClear(kc *keychain.Client, account string) {
	in := bufio.NewReader(os.Stdin)
	if !promptYesNo(in, "⚠ this removes the bot token; the daemon will refuse to start. Continue? [y/N] ", false) {
		fmt.Println("aborted")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kc.Delete(ctx, keychain.ServiceToken, account); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		fatalf("delete token: %v", err)
	}
	fmt.Println("✓ token cleared")
}

// tokenReauth re-writes the Keychain entry with a fresh
// trusted-binary list, granting silent-read access to the
// CURRENT macontrol binary. Used when the binary has moved
// between locations (Homebrew relocate, switch from manual
// install to brew, or vice versa) and macOS has started
// prompting for the entry's ACL on every daemon read.
//
// Behavior:
//  1. Resolve the current binary path via [os.Executable] (in
//     [runToken]); [fatalf] if it's empty.
//  2. Read the existing token from [keychain.ServiceToken].
//     [fatalf] on read failure (typically: keychain locked
//     or no token stored).
//  3. Re-write the token via [keychain.Client.Set] with the
//     resolved binary in the trusted-binary list. Macos
//     re-issues the entry's ACL.
//  4. Best-effort do the same for [keychain.ServiceWhitelist]
//     so the whitelist's ACL stays consistent. Errors are
//     swallowed because a missing whitelist isn't a fatal
//     reauth failure.
//  5. Print success + a "daemon will read silently again"
//     reminder.
//
// Does NOT verify the token via getMe — the user is just
// re-granting ACL, not changing the secret.
func tokenReauth(kc *keychain.Client, account, exe string) {
	if exe == "" {
		fatalf("could not resolve macontrol binary path")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tok, err := kc.Get(ctx, keychain.ServiceToken, account)
	if err != nil {
		fatalf("read existing token: %v", err)
	}
	if err := kc.Set(ctx, keychain.ServiceToken, account, tok, exe); err != nil {
		fatalf("re-issue token entry: %v", err)
	}
	// Same for whitelist (less critical but keeps the ACL consistent).
	if raw, err := kc.Get(ctx, keychain.ServiceWhitelist, account); err == nil {
		_ = kc.Set(ctx, keychain.ServiceWhitelist, account, raw, exe)
	}
	fmt.Printf("✓ Keychain ACL refreshed for %s\n", exe)
	fmt.Println("the daemon will read silently again on next start")
}
