package flows_test

import (
	"context"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

func newSoundSvc(_ *testing.T) *sound.Service {
	f := runner.NewFake()
	f.On("osascript -e set volume output volume 42", "", nil)
	f.On(
		"osascript -e set v to output volume of (get volume settings)\n"+
			"set m to output muted of (get volume settings)\n"+
			"return (v as text) & \",\" & (m as text)",
		"42,false", nil)
	return sound.New(f)
}

func TestSetVolumeFlow_RejectsNonInt(t *testing.T) {
	t.Parallel()
	f := flows.NewSetVolume(newSoundSvc(t))
	resp := f.Handle(context.Background(), "not a number")
	if resp.Done {
		t.Fatal("flow should stay open on invalid input")
	}
}

func TestSetVolumeFlow_RejectsOutOfRange(t *testing.T) {
	t.Parallel()
	f := flows.NewSetVolume(newSoundSvc(t))
	resp := f.Handle(context.Background(), "150")
	if resp.Done {
		t.Fatal("flow should stay open for out-of-range values")
	}
}

func TestSetVolumeFlow_AcceptsValidNumber(t *testing.T) {
	t.Parallel()
	f := flows.NewSetVolume(newSoundSvc(t))
	resp := f.Handle(context.Background(), "42")
	if !resp.Done {
		t.Fatal("flow should finish on valid input")
	}
	if resp.Text == "" {
		t.Fatal("expected reply text")
	}
}
