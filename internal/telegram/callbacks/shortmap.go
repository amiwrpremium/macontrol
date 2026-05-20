package callbacks

import (
	"crypto/rand"
	"encoding/base32"
	"sync"
	"time"
)

// ShortMap is a TTL-bounded side table for callback-arg values
// that are too long to embed in the 64-byte
// [MaxCallbackDataBytes] budget directly.
//
// Five users in the codebase right now: Bluetooth MACs (paired
// device list), Wi-Fi SSIDs (join + DNS reset), disk mount paths
// (Disks panel), Shortcuts names (Run Shortcut picker), IANA
// timezone names (Timezone picker). Each callback embeds only a
// short [Put]-returned id; the handler resolves it back via
// [Get] before acting.
//
// Lifecycle:
//   - Constructed once at daemon startup via [NewShortMap], stored
//     on bot.Deps.ShortMap, and shared by every keyboard +
//     handler for the process lifetime.
//   - Background eviction is opt-in via [StartJanitor]; without it,
//     entries are pruned lazily on [Get] miss-after-expiry only.
//   - Tests may pin time by overriding [ShortMap.now] (see
//     shortmap_test.go).
//
// Concurrency:
//   - All access is serialised through mu. Safe to call any method
//     from any goroutine, including from the janitor goroutine
//     started by [StartJanitor].
//
// Field roles:
//   - ttl is the per-entry expiry budget, applied at Put time
//     (each entry expires at now+ttl). Falls back to 15 minutes
//     when [NewShortMap] receives <= 0.
//   - items is the dispatch table keyed by the random id [newID]
//     produces.
//   - now is the time source — pinned to time.Now in production,
//     overridden in tests for deterministic expiry.
type ShortMap struct {
	// ttl is the per-entry lifetime applied at [Put] time.
	ttl time.Duration

	// mu serialises every field access including the now lookup.
	mu sync.Mutex

	// items maps id → (value, expiry). Pruned lazily by [Get] and
	// proactively by [Sweep] / the [StartJanitor] goroutine.
	items map[string]shortItem

	// now is the wall-clock source. Pinned to time.Now in
	// production; tests inject a stub for deterministic expiry.
	now func() time.Time
}

// shortItem is one entry in [ShortMap.items]. Internal because
// callers should never see expired entries — [Get] either returns
// the value or admits "not found".
type shortItem struct {
	// value is the original full string the keyboard handed to
	// [ShortMap.Put].
	value string

	// expires is the absolute deadline after which [Get] treats
	// the entry as missing. Set at Put time to now+ttl.
	expires time.Time
}

// NewShortMap returns a [ShortMap] with the given TTL applied to
// every entry it stores. A non-positive ttl is replaced with the
// safe default of 15 minutes.
//
// Behavior:
//   - Initialises items as an empty map and now as time.Now.
//   - Does NOT spawn the eviction goroutine; callers that want
//     proactive pruning must call [ShortMap.StartJanitor]
//     separately. Without it, expired entries linger in memory
//     until something attempts to [Get] them.
//
// Returns a *[ShortMap] ready for concurrent use. Never returns
// an error.
func NewShortMap(ttl time.Duration) *ShortMap {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ShortMap{
		ttl:   ttl,
		items: map[string]shortItem{},
		now:   time.Now,
	}
}

// Put stores value under a freshly-generated short id and returns
// the id. The id is 10 base32 chars (~50 bits of entropy) — short
// enough to comfortably fit inside the 64-byte
// [MaxCallbackDataBytes] budget alongside the namespace, action,
// and any other args.
//
// Behavior:
//   - Generates a fresh id via [newID]; collisions are
//     astronomically unlikely but if one occurs the existing
//     entry is overwritten silently.
//   - Marks the entry to expire at now()+ttl using the receiver's
//     time source.
//
// Returns the id string. Never returns an error.
func (m *ShortMap) Put(value string) string {
	id := newID()
	m.mu.Lock()
	m.items[id] = shortItem{value: value, expires: m.now().Add(m.ttl)}
	m.mu.Unlock()
	return id
}

// Get resolves an id back to the value originally stored by
// [ShortMap.Put]. Handlers call this on every callback that
// carries a short-id arg.
//
// Behavior:
//   - Returns ("", false) when the id is unknown.
//   - Returns ("", false) AND deletes the entry when the id is
//     known but its expiry has passed (lazy eviction).
//   - Returns (value, true) otherwise.
//
// The handler is expected to render a "session expired — refresh
// the list" message on the (id known, expired) and (id unknown)
// cases — both look the same from this method's return.
func (m *ShortMap) Get(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok {
		return "", false
	}
	if m.now().After(it.expires) {
		delete(m.items, id)
		return "", false
	}
	return it.value, true
}

// Sweep removes every expired entry in one pass. Safe to call
// periodically (the [StartJanitor] goroutine does exactly that).
//
// Behavior:
//   - Snapshots time once via the receiver's now source, then
//     deletes any entry whose expires field is before that
//     snapshot.
//   - Holds the mutex for the full pass; the cost is O(n) over
//     the current entry count.
func (m *ShortMap) Sweep() {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, it := range m.items {
		if now.After(it.expires) {
			delete(m.items, id)
		}
	}
}

// Size returns the current entry count, including not-yet-pruned
// expired entries. Provided for tests and debug instrumentation;
// production code shouldn't rely on it for correctness.
func (m *ShortMap) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// StartJanitor spawns a background goroutine that calls
// [ShortMap.Sweep] every ttl/2 until stop is closed. Returns
// immediately.
//
// Lifecycle:
//   - Daemon startup wires this to a process-lifetime stop
//     channel so the janitor lives as long as the process.
//   - Tests pass a short-lived stop channel to bound the
//     goroutine's lifetime to the test.
//
// Behavior:
//   - Ticks every ttl/2 (so an entry is evicted within at most
//     1.5 × ttl after creation in the worst case).
//   - Stops cleanly when stop is closed; ticker is released via
//     defer so no resource leaks.
//   - Each tick calls [ShortMap.Sweep] under the mutex; concurrent
//     Get/Put callers serialise behind it.
func (m *ShortMap) StartJanitor(stop <-chan struct{}) {
	tick := time.NewTicker(m.ttl / 2)
	go func() {
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				m.Sweep()
			case <-stop:
				return
			}
		}
	}()
}

// newID returns a fresh base32-encoded random id of 10 chars
// (5 random bytes, ~50 bits of entropy). Padding is stripped so
// the id is human-friendly and stays short.
//
// Behavior:
//   - Uses crypto/rand.Read; failures are intentionally ignored
//     because rand.Read on macOS/Linux only ever returns nil in
//     practice. A misread would yield a zero-bytes id, which is
//     fine — a collision with another zero-bytes id just causes
//     the older entry to be overwritten.
func newID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
