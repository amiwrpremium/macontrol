package callbacks

import (
	"crypto/rand"
	"encoding/base32"
	"sync"
	"time"
)

// ShortMap stores callback-arg values that are too long to embed in the
// 64-byte callback_data string (Bluetooth MACs, SSIDs, timezone names).
// Keys are short opaque ids with a TTL.
type ShortMap struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]shortItem
	now   func() time.Time
}

type shortItem struct {
	value   string
	expires time.Time
}

// NewShortMap returns a ShortMap with the given TTL. A goroutine janitor
// evicts expired entries every ttl/2.
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

// Put stores value and returns an id that can be placed in callback_data.
// The id is 10 base32 chars (50 bits of entropy).
func (m *ShortMap) Put(value string) string {
	id := newID()
	m.mu.Lock()
	m.items[id] = shortItem{value: value, expires: m.now().Add(m.ttl)}
	m.mu.Unlock()
	return id
}

// Get looks up the id. Returns ("", false) if missing or expired.
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

// Sweep removes expired entries. Safe to call periodically.
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

// Size returns the current count (for tests/metrics).
func (m *ShortMap) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// StartJanitor spawns a goroutine that calls Sweep every ttl/2 until stop
// is closed. Returns immediately.
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

func newID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
