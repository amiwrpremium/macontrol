package bot

import "github.com/go-telegram/bot/models"

// Whitelist is the auth boundary: a set of Telegram user IDs
// whose updates the daemon will process. Anything from a
// non-whitelisted sender is dropped silently in [Deps.dispatch]
// before any handler runs.
//
// The map shape (`map[int64]struct{}`) gives O(1) membership
// checks — important because Allows is called on every
// incoming update on the dispatcher's hot path.
//
// Lifecycle:
//   - Built once at daemon startup from the parsed
//     ALLOWED_USER_IDS Keychain entry (see [config.Config.AllowedUserIDs]
//     and [NewWhitelist]).
//   - Stored on [Deps.Whitelist]; never mutated thereafter.
//     Adding/removing IDs requires editing the Keychain
//     entry via `macontrol whitelist add/remove` and
//     restarting the daemon.
//
// Concurrency:
//   - Read-only after construction → safe for concurrent
//     reads from any goroutine without synchronisation.
type Whitelist map[int64]struct{}

// NewWhitelist constructs a [Whitelist] from a slice of
// permitted user IDs. Duplicates in ids are silently merged
// (set semantics).
func NewWhitelist(ids []int64) Whitelist {
	w := Whitelist{}
	for _, id := range ids {
		w[id] = struct{}{}
	}
	return w
}

// Allows reports whether update's sender is permitted to
// interact with the bot.
//
// Behavior:
//   - Resolves the sender via [senderID].
//   - Returns false when senderID returns 0 (no recognisable
//     sender — defensive against update kinds we don't
//     handle).
//   - Returns true when the sender ID is in the whitelist
//     map.
//
// Called by [Deps.dispatch] on every update before handler
// dispatch.
func (w Whitelist) Allows(update *models.Update) bool {
	id := senderID(update)
	if id == 0 {
		return false
	}
	_, ok := w[id]
	return ok
}

// Members returns every whitelisted ID as an int64 slice.
// Order is NOT stable (Go map iteration is randomised);
// callers that need sorted output should sort the result.
//
// Used by [pingOnBoot] to send the boot ping to every user
// (where order doesn't matter).
func (w Whitelist) Members() []int64 {
	out := make([]int64, 0, len(w))
	for id := range w {
		out = append(out, id)
	}
	return out
}

// senderID extracts the Telegram user ID of u's sender.
//
// Routing rules (first match wins):
//  1. u.CallbackQuery non-nil → return CallbackQuery.From.ID.
//  2. u.Message non-nil with non-nil From → return
//     Message.From.ID.
//  3. Otherwise → return 0 (sentinel "no sender" value
//     [Whitelist.Allows] uses to reject).
//
// The two recognised cases cover every user-initiated
// interaction macontrol cares about (taps and typed
// messages). Edited messages, channel posts, etc. fall
// through to 0 — silently dropped at the auth gate.
func senderID(u *models.Update) int64 {
	switch {
	case u.CallbackQuery != nil:
		return u.CallbackQuery.From.ID
	case u.Message != nil && u.Message.From != nil:
		return u.Message.From.ID
	}
	return 0
}
