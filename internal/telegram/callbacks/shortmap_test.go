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
