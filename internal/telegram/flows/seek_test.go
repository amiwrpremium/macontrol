package flows_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// musicSvcOK returns a music.Service whose Seek call against
// the given seconds value will succeed.
func musicSvcOK(secs string) *music.Service {
	return music.New(runner.NewFake().On("nowplaying-cli seek "+secs, "", nil))
}

func TestNewSeek_Name(t *testing.T) {
	t.Parallel()
	f := flows.NewSeek(musicSvcOK("0"))
	if f.Name() != "mus:seek" {
		t.Fatalf("name = %q", f.Name())
	}
}

func TestNewSeek_StartHasRangeHint(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("0")).Start(context.Background())
	if !strings.Contains(r.Text, "0") || !strings.Contains(r.Text, "86400") {
		t.Errorf("start text missing range: %q", r.Text)
	}
	if r.Done {
		t.Error("start must not terminate the flow")
	}
}

func TestNewSeek_HandleHappyPath(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("90")).Handle(context.Background(), "90")
	if !r.Done {
		t.Error("expected Done on success")
	}
	if !strings.Contains(r.Text, "1:30") {
		t.Errorf("expected formatted position 1:30 in text; got %q", r.Text)
	}
}

func TestNewSeek_HandleHourLong(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("3661")).Handle(context.Background(), "3661")
	if !strings.Contains(r.Text, "1:01:01") {
		t.Errorf("expected h:mm:ss formatting for 3661s; got %q", r.Text)
	}
}

func TestNewSeek_HandleNonNumericReprompts(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("0")).Handle(context.Background(), "banana")
	if r.Done {
		t.Error("re-prompt must NOT terminate the flow")
	}
	if !strings.Contains(r.Text, "0..86400") {
		t.Errorf("expected re-prompt range hint; got %q", r.Text)
	}
}

func TestNewSeek_HandleNegativeReprompts(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("0")).Handle(context.Background(), "-5")
	if r.Done {
		t.Error("re-prompt must NOT terminate the flow on negative input")
	}
}

func TestNewSeek_HandleOverflowReprompts(t *testing.T) {
	t.Parallel()
	r := flows.NewSeek(musicSvcOK("0")).Handle(context.Background(), "100000")
	if r.Done {
		t.Error("re-prompt must NOT terminate the flow on out-of-range input")
	}
}

func TestNewSeek_HandleSeekError(t *testing.T) {
	t.Parallel()
	bad := music.New(runner.NewFake().On("nowplaying-cli seek 30", "", errors.New("not seekable")))
	r := flows.NewSeek(bad).Handle(context.Background(), "30")
	if !r.Done {
		t.Error("expected Done after seek error")
	}
	if !strings.Contains(r.Text, "could not seek") {
		t.Errorf("expected error banner; got %q", r.Text)
	}
}
