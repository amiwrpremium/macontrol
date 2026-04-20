package flows

import (
	"context"
	"sync"
	"time"
)

// Registry tracks the active Flow for each chat. Inactive flows expire.
type Registry struct {
	ttl  time.Duration
	mu   sync.Mutex
	live map[int64]*entry
	now  func() time.Time
}

type entry struct {
	flow      Flow
	lastTouch time.Time
}

// NewRegistry returns a Registry with the given inactivity TTL.
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

// Install adds a new flow for chatID, replacing any in-progress flow.
func (r *Registry) Install(chatID int64, f Flow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.live[chatID] = &entry{flow: f, lastTouch: r.now()}
}

// Active returns the flow for chatID if there is one and it has not timed
// out, refreshing its last-touch timestamp.
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

// Cancel clears any flow for chatID. Returns true if one was present.
func (r *Registry) Cancel(chatID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.live[chatID]
	delete(r.live, chatID)
	return ok
}

// Finish is called after a flow signals Done.
func (r *Registry) Finish(chatID int64) { r.Cancel(chatID) }

// Sweep evicts timed-out flows. Safe to call from a janitor goroutine.
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

// StartJanitor runs Sweep every ttl/2 until stop closes.
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

// Size returns the current number of active flows (for tests/metrics).
func (r *Registry) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.live)
}
