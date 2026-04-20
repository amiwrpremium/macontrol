package flows

import (
	"context"
	"testing"
	"time"
)

type stubFlow struct{ name string }

func (s stubFlow) Name() string                              { return s.name }
func (s stubFlow) Start(_ context.Context) Response          { return Response{} }
func (s stubFlow) Handle(_ context.Context, _ string) Response {
	return Response{Done: true}
}

func TestRegistry_InstallActiveCancel(t *testing.T) {
	t.Parallel()
	r := NewRegistry(time.Minute)
	r.Install(42, stubFlow{name: "test"})
	if _, ok := r.Active(42); !ok {
		t.Fatal("flow should be active")
	}
	if !r.Cancel(42) {
		t.Fatal("cancel should return true")
	}
	if _, ok := r.Active(42); ok {
		t.Fatal("flow should be gone after cancel")
	}
}

func TestRegistry_TimesOut(t *testing.T) {
	t.Parallel()
	r := NewRegistry(time.Minute)
	base := time.Now()
	r.now = func() time.Time { return base }
	r.Install(42, stubFlow{name: "test"})
	r.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, ok := r.Active(42); ok {
		t.Fatal("flow should have timed out")
	}
}

func TestRegistry_Sweep(t *testing.T) {
	t.Parallel()
	r := NewRegistry(time.Minute)
	base := time.Now()
	r.now = func() time.Time { return base }
	for i := int64(0); i < 5; i++ {
		r.Install(i, stubFlow{name: "t"})
	}
	if r.Size() != 5 {
		t.Fatalf("size = %d", r.Size())
	}
	r.now = func() time.Time { return base.Add(2 * time.Minute) }
	r.Sweep()
	if r.Size() != 0 {
		t.Fatalf("after sweep size = %d", r.Size())
	}
}
