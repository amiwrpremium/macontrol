package callbacks

import (
	"testing"
	"time"
)

func TestShortMap_PutGet(t *testing.T) {
	t.Parallel()
	m := NewShortMap(time.Minute)
	id := m.Put("Europe/Istanbul")
	if len(id) == 0 || len(id) > 12 {
		t.Fatalf("unexpected id length: %q", id)
	}
	v, ok := m.Get(id)
	if !ok || v != "Europe/Istanbul" {
		t.Fatalf("lookup miss: v=%q ok=%v", v, ok)
	}
}

func TestShortMap_Expires(t *testing.T) {
	t.Parallel()
	m := NewShortMap(time.Minute)
	// Override clock.
	base := time.Now()
	m.now = func() time.Time { return base }
	id := m.Put("x")
	m.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, ok := m.Get(id); ok {
		t.Fatal("expected expired entry to be gone")
	}
}

func TestShortMap_Sweep(t *testing.T) {
	t.Parallel()
	m := NewShortMap(time.Minute)
	base := time.Now()
	m.now = func() time.Time { return base }
	for i := 0; i < 5; i++ {
		m.Put("v")
	}
	if m.Size() != 5 {
		t.Fatalf("size = %d", m.Size())
	}
	m.now = func() time.Time { return base.Add(2 * time.Minute) }
	m.Sweep()
	if m.Size() != 0 {
		t.Fatalf("after sweep size = %d", m.Size())
	}
}

func TestShortMap_UniqueIDs(t *testing.T) {
	t.Parallel()
	m := NewShortMap(time.Minute)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := m.Put("v")
		if seen[id] {
			t.Fatalf("collision at iteration %d", i)
		}
		seen[id] = true
	}
}

func TestShortMap_NewShortMap_DefaultsZeroTTL(t *testing.T) {
	t.Parallel()
	// Zero TTL should fall back to the 15-minute default — entries
	// inserted just now must still be retrievable.
	m := NewShortMap(0)
	id := m.Put("hello")
	got, ok := m.Get(id)
	if !ok || got != "hello" {
		t.Fatalf("zero-TTL should default to 15m; got=%q ok=%v", got, ok)
	}
}

func TestShortMap_Get_MissingID(t *testing.T) {
	t.Parallel()
	m := NewShortMap(time.Minute)
	if _, ok := m.Get("never-existed"); ok {
		t.Fatal("expected miss")
	}
}

func TestShortMap_StartJanitor_EvictsExpiredEntries(t *testing.T) {
	t.Parallel()
	// Use a real-time TTL of 10ms so the janitor (running every TTL/2 = 5ms)
	// will sweep within the test window.
	m := NewShortMap(10 * time.Millisecond)
	id := m.Put("ephemeral")

	stop := make(chan struct{})
	defer close(stop)
	m.StartJanitor(stop)

	// Wait long enough for the entry to expire AND for the janitor to sweep.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := m.Get(id); !ok {
			return // success
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("janitor did not evict expired entry within 500ms")
}

func TestShortMap_StartJanitor_StopsCleanly(t *testing.T) {
	t.Parallel()
	m := NewShortMap(20 * time.Millisecond)
	stop := make(chan struct{})
	m.StartJanitor(stop)
	// Closing the stop channel must not panic and the goroutine should
	// exit promptly. We can't easily observe the goroutine directly, so
	// just close-and-wait briefly to assert no panic.
	close(stop)
	time.Sleep(50 * time.Millisecond)
}
