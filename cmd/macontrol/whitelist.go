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

// runWhitelist is the implementation of `macontrol whitelist
// {list|add|remove|clear}`. Manages the comma-separated user-ID
// whitelist stored in the [keychain.ServiceWhitelist] entry.
//
// Routing rules (args[0] — first match wins):
//  1. "list"          → [whitelistList].
//  2. "add <id>"      → [whitelistAdd] (rejects duplicates).
//  3. "remove" / "rm" → [whitelistRemove] (refuses to remove
//     the last entry — use clear).
//  4. "clear"         → [whitelistClear] (asks for
//     confirmation; daemon rejects all updates afterwards).
//
// Empty args → usage line + exit 2. Unknown subcommand →
// [fatalf] (exits 1). Every mutation prints a "restart the
// daemon for changes to take effect" reminder because the
// whitelist is loaded once at boot and not re-read.
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

// whitelistRead fetches and parses the current whitelist from
// the [keychain.ServiceWhitelist] entry.
//
// Behavior:
//   - 5-second context timeout — keychain reads should not
//     hang.
//   - Returns ([]int64{}, nil) when the entry is missing
//     (ErrNotFound). The "not present" case is treated as
//     "empty whitelist" rather than a hard failure so the
//     CLI can still add the first entry without a separate
//     init step.
//   - Returns the parsed slice on success, or the keychain /
//     parse error verbatim otherwise.
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

// whitelistWrite serialises ids via [config.FormatUserIDs] and
// stores them in the [keychain.ServiceWhitelist] entry,
// preserving the macontrol-binary trust grant so the daemon
// can read silently after the first prompt.
//
// Behavior:
//   - 5-second context timeout.
//   - Includes exe in the trusted-binaries list (when
//     non-empty) so [keychain.Client.Set] preserves silent-
//     read access.
func whitelistWrite(kc *keychain.Client, account, exe string, ids []int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	trusted := []string{}
	if exe != "" {
		trusted = append(trusted, exe)
	}
	return kc.Set(ctx, keychain.ServiceWhitelist, account, config.FormatUserIDs(ids), trusted...)
}

// whitelistList prints every whitelisted user ID, one per
// line. On empty whitelist, prints "(empty)" so the user sees
// confirmation rather than a silent no-op.
//
// Behavior:
//   - Calls [whitelistRead]; [fatalf]s on read failure.
//   - Prints "(empty)" when the parsed slice has zero
//     entries.
//   - Otherwise prints each int64 on its own line.
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

// whitelistAdd appends raw (parsed as int64) to the whitelist,
// rejecting duplicates without modifying state.
//
// Behavior:
//  1. Parse raw as int64 — [fatalf] on non-numeric.
//  2. Read existing whitelist via [whitelistRead].
//  3. If id already present, prints "X already whitelisted"
//     and returns without writing.
//  4. Otherwise append + write back via [whitelistWrite].
//  5. Prints success line + the standard "restart the daemon"
//     reminder.
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

// whitelistRemove removes raw (parsed as int64) from the
// whitelist.
//
// Behavior:
//  1. Parse raw as int64 — [fatalf] on non-numeric.
//  2. Read existing whitelist via [whitelistRead].
//  3. Walk the slice; skip the entry matching id, copy the
//     rest into out.
//  4. If nothing was removed, prints "X not on the whitelist;
//     no change" and returns.
//  5. If the result would be EMPTY, [fatalf]s with a hint to
//     use `whitelist clear` instead. Defends against
//     accidentally locking everyone out.
//  6. Otherwise writes the trimmed slice back and prints the
//     "restart the daemon" reminder.
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

// whitelistClear deletes the entire [keychain.ServiceWhitelist]
// entry after a y/N prompt. The daemon's auth gate then rejects
// every incoming update, effectively muting the bot until a new
// entry is added.
//
// Behavior:
//  1. Reads y/N via [promptYesNo] with a default of N (so
//     bare-Enter aborts).
//  2. Calls [keychain.Client.Delete]; treats ErrNotFound as
//     success (already empty).
//  3. Prints success + a "restart the daemon" reminder.
//
// Used only as the explicit escape hatch when the user really
// wants to lock out the bot — the standard "remove last entry"
// path is intentionally blocked by [whitelistRemove].
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
