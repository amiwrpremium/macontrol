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

// tokenReauth re-issues the Keychain entry with a fresh -T <macontrol-binary>
// argument. Useful when the binary moved (brew relocated, switched between
// brew and manual install) and macOS started prompting for the existing
// entry's ACL on every read.
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
