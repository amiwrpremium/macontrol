package flows

import (
	"context"
	"sync"
	"time"
)

// Registry is the per-chat active-flow store. One instance lives
// on bot.Deps.FlowReg for the process lifetime; the bot's
// dispatcher consults it on every plain-text message to route the
// reply to the chat's in-progress [Flow].
//
// The registry implements the bot.FlowManager interface (Active /
// Cancel / Install / Finish), plus extras (Sweep / StartJanitor /
// Size) used by daemon startup and tests.
//
// Lifecycle:
//   - Constructed once at daemon startup via [NewRegistry] with a
//     per-chat inactivity TTL.
//   - [Registry.Install] wires a flow into a chat slot when the
//     handler that opens the flow runs.
//   - [Registry.Active] is called on every plain-text message;
//     touch refreshes the entry's deadline.
//   - [Registry.Finish] (or [Registry.Cancel]) unwires the slot.
//   - [Registry.Sweep] / [Registry.StartJanitor] proactively
//     evict flows the user abandoned without sending /cancel.
//
// Concurrency:
//   - All access is serialised through mu. Safe for concurrent
//     calls from multiple goroutines (the dispatcher, command
//     handlers, the janitor).
//
// Field roles:
//   - ttl is the per-flow inactivity budget; an entry whose
//     lastTouch is older than now-ttl is treated as gone by
//     Active and removed by Sweep.
//   - live is the dispatch table keyed by chat ID.
//   - now is the wall-clock source. Pinned to time.Now in
//     production; tests inject a stub for deterministic expiry.
type Registry struct {
	// ttl is the per-chat inactivity budget. Defaults to 5 min
	// when [NewRegistry] receives <= 0.
	ttl time.Duration

	// mu serialises every access to live.
	mu sync.Mutex

	// live maps chatID → entry. Pruned lazily by [Registry.Active]
	// on miss-after-expiry and proactively by [Registry.Sweep].
	live map[int64]*entry

	// now is the wall-clock source. Pinned to time.Now in
	// production; tests stub this for deterministic expiry.
	now func() time.Time
}

// entry is one chat's active-flow slot. Internal because callers
// only ever interact with the [Flow] itself; the lastTouch is a
// registry-private bookkeeping field.
type entry struct {
	// flow is the in-progress [Flow] for the chat.
	flow Flow

	// lastTouch is when the entry was created or last accessed
	// via [Registry.Active]. Compared against now-ttl to detect
	// inactivity.
	lastTouch time.Time
}

// NewRegistry returns a [Registry] with the given inactivity ttl
// applied to every chat slot. A non-positive ttl is replaced with
// the safe default of 5 minutes.
//
// Behavior:
//   - Initialises live as an empty map and now as time.Now.
//   - Does NOT spawn the eviction goroutine; callers that want
//     proactive pruning must call [Registry.StartJanitor]
//     separately. Without it, expired flows linger in memory
//     until [Registry.Active] next touches them.
//
// Returns a *[Registry] ready for concurrent use.
func NewRegistry(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Registry{
		ttl:  ttl,
		live: map[int64]*entry{},
		now:  time.Now,
	}
}

// Install registers f as the active flow for chatID. Replaces any
// in-progress flow without notifying it (no Cancel hook on the
// old flow; see the smells list).
//
// Behavior:
//   - Stamps the entry with the current now() so the deadline is
//     ttl-from-now.
//   - Last-write-wins: if two handlers race to install for the
//     same chat, the second silently wins.
func (r *Registry) Install(chatID int64, f Flow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.live[chatID] = &entry{flow: f, lastTouch: r.now()}
}

// Active looks up the in-progress flow for chatID, refreshing the
// entry's lastTouch as a side effect so the TTL is rolling rather
// than absolute.
//
// Behavior:
//   - Returns (nil, false) when no flow is registered for the
//     chat.
//   - Returns (nil, false) AND deletes the entry when the flow is
//     registered but its lastTouch is older than now-ttl (lazy
//     eviction).
//   - Returns (flow, true) on hit and refreshes lastTouch to now.
func (r *Registry) Active(chatID int64) (Flow, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.live[chatID]
	if !ok {
		return nil, false
	}
	if r.now().Sub(e.lastTouch) > r.ttl {
		delete(r.live, chatID)
		return nil, false
	}
	e.lastTouch = r.now()
	return e.flow, true
}

// Cancel removes any flow registered for chatID. Used by the
// /cancel command (which the bot dispatcher consumes before the
// flow's Handle ever sees it).
//
// Returns true when a flow was actually present and removed,
// false when there was nothing to cancel.
func (r *Registry) Cancel(chatID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.live[chatID]
	delete(r.live, chatID)
	return ok
}

// Finish removes the active flow for chatID. Called by the
// dispatcher after a [Flow.Handle] returns Response{Done: true}.
// Distinct from [Registry.Cancel] only in name and intent —
// implementation is identical (delegated to Cancel) — see the
// smells list on bot.go for the redundant API surface.
func (r *Registry) Finish(chatID int64) { r.Cancel(chatID) }

// Sweep evicts every chat whose flow has been idle longer than
// ttl. Safe to call periodically (the [Registry.StartJanitor]
// goroutine does exactly that).
//
// Behavior:
//   - Snapshots time once via the receiver's now source, then
//     deletes any entry whose lastTouch is older than now-ttl.
//   - Holds the mutex for the full pass (O(n) over current
//     entries).
func (r *Registry) Sweep() {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for chatID, e := range r.live {
		if now.Sub(e.lastTouch) > r.ttl {
			delete(r.live, chatID)
		}
	}
}

// StartJanitor spawns a background goroutine that calls
// [Registry.Sweep] every ttl/2 until ctx is cancelled. Returns
// immediately.
//
// Lifecycle:
//   - Daemon startup wires this to the process-lifetime context.
//   - Tests pass a short-lived context (cancelled in t.Cleanup)
//     to bound the goroutine.
//
// Behavior:
//   - Ticks every ttl/2 (so an idle flow is evicted within at
//     most 1.5 × ttl after its last touch in the worst case).
//   - Stops cleanly when ctx is cancelled; ticker is released via
//     defer so no resource leaks.
//   - The signature takes context.Context whereas [callbacks.ShortMap]
//     takes a `<-chan struct{}` for the same purpose — see the
//     smells list for the inconsistency.
func (r *Registry) StartJanitor(ctx context.Context) {
	tick := time.NewTicker(r.ttl / 2)
	go func() {
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				r.Sweep()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Size returns the current count of active flows, including
// not-yet-pruned expired entries. Provided for tests and debug
// instrumentation; production code should not depend on it for
// correctness.
func (r *Registry) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.live)
}
