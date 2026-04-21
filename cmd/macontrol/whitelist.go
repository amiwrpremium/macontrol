package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/config"
	"github.com/amiwrpremium/macontrol/internal/keychain"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func runWhitelist(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: macontrol whitelist {list|add ID|remove ID|clear}")
		os.Exit(2)
	}
	kc := keychain.New(runner.New())
	account := currentUser()
	exe, _ := os.Executable()

	switch args[0] {
	case "list":
		whitelistList(kc, account)
	case "add":
		if len(args) < 2 {
			fatalf("usage: macontrol whitelist add <userid>")
		}
		whitelistAdd(kc, account, exe, args[1])
	case "remove", "rm":
		if len(args) < 2 {
			fatalf("usage: macontrol whitelist remove <userid>")
		}
		whitelistRemove(kc, account, exe, args[1])
	case "clear":
		whitelistClear(kc, account)
	default:
		fatalf("unknown whitelist subcommand: %s", args[0])
	}
}

func whitelistRead(kc *keychain.Client, account string) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := kc.Get(ctx, keychain.ServiceWhitelist, account)
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return []int64{}, nil
		}
		return nil, err
	}
	return config.ParseUserIDs(raw)
}

func whitelistWrite(kc *keychain.Client, account, exe string, ids []int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	trusted := []string{}
	if exe != "" {
		trusted = append(trusted, exe)
	}
	return kc.Set(ctx, keychain.ServiceWhitelist, account, config.FormatUserIDs(ids), trusted...)
}

func whitelistList(kc *keychain.Client, account string) {
	ids, err := whitelistRead(kc, account)
	if err != nil {
		fatalf("read whitelist: %v", err)
	}
	if len(ids) == 0 {
		fmt.Println("(empty)")
		return
	}
	for _, id := range ids {
		fmt.Println(id)
	}
}

func whitelistAdd(kc *keychain.Client, account, exe, raw string) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		fatalf("user id must be an integer: %v", err)
	}
	ids, err := whitelistRead(kc, account)
	if err != nil {
		fatalf("read whitelist: %v", err)
	}
	for _, existing := range ids {
		if existing == id {
			fmt.Printf("%d already whitelisted\n", id)
			return
		}
	}
	ids = append(ids, id)
	if err := whitelistWrite(kc, account, exe, ids); err != nil {
		fatalf("write whitelist: %v", err)
	}
	fmt.Printf("✓ added %d (whitelist now has %d entries)\n", id, len(ids))
	fmt.Println("restart the daemon for changes to take effect: brew services restart macontrol")
}

func whitelistRemove(kc *keychain.Client, account, exe, raw string) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		fatalf("user id must be an integer: %v", err)
	}
	ids, err := whitelistRead(kc, account)
	if err != nil {
		fatalf("read whitelist: %v", err)
	}
	out := make([]int64, 0, len(ids))
	removed := false
	for _, existing := range ids {
		if existing == id {
			removed = true
			continue
		}
		out = append(out, existing)
	}
	if !removed {
		fmt.Printf("%d not on the whitelist; no change\n", id)
		return
	}
	if len(out) == 0 {
		fatalf("refusing to remove the last whitelist entry — use `macontrol whitelist clear` if you really want this")
	}
	if err := whitelistWrite(kc, account, exe, out); err != nil {
		fatalf("write whitelist: %v", err)
	}
	fmt.Printf("✓ removed %d (whitelist now has %d entries)\n", id, len(out))
	fmt.Println("restart the daemon for changes to take effect: brew services restart macontrol")
}

func whitelistClear(kc *keychain.Client, account string) {
	in := bufio.NewReader(os.Stdin)
	if !promptYesNo(in, "⚠ this empties the whitelist; the daemon will reject all updates. Continue? [y/N] ", false) {
		fmt.Println("aborted")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kc.Delete(ctx, keychain.ServiceWhitelist, account); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		fatalf("delete whitelist: %v", err)
	}
	fmt.Println("✓ whitelist cleared")
	fmt.Println("restart the daemon for changes to take effect")
}
