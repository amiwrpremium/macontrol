package sound_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

const getScript = "osascript -e set v to output volume of (get volume settings)\n" +
	"set m to output muted of (get volume settings)\n" +
	"return (v as text) & \",\" & (m as text)"

func fake(canned string) *runner.Fake {
	f := runner.NewFake().On(getScript, canned, nil)
	return f
}

func TestGet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stdout string
		want   sound.State
		ok     bool
	}{
		{"unmuted", "60,false", sound.State{Level: 60, Muted: false}, true},
		{"muted", "25,true", sound.State{Level: 25, Muted: true}, true},
		{"zero", "0,false", sound.State{Level: 0, Muted: false}, true},
		{"over clamps", "150,false", sound.State{Level: 100, Muted: false}, true},
		{"negative clamps", "-5,false", sound.State{Level: 0, Muted: false}, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			svc := sound.New(fake(c.stdout))
			st, err := svc.Get(context.Background())
			if (err == nil) != c.ok {
				t.Fatalf("err=%v ok=%v", err, c.ok)
			}
			if st != c.want {
				t.Errorf("got %+v want %+v", st, c.want)
			}
		})
	}
}

func TestGet_UnexpectedFormat(t *testing.T) {
	t.Parallel()
	svc := sound.New(runner.NewFake().On(getScript, "bogus", nil))
	if _, err := svc.Get(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGet_NonIntegerVolume(t *testing.T) {
	t.Parallel()
	svc := sound.New(runner.NewFake().On(getScript, "notanint,false", nil))
	if _, err := svc.Get(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGet_OsascriptFails(t *testing.T) {
	t.Parallel()
	svc := sound.New(runner.NewFake().On(getScript, "", errors.New("osascript boom")))
	if _, err := svc.Get(context.Background()); err == nil {
		t.Fatal("expected bubbled error")
	}
}

func TestSet_ClampsAndForwards(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, sent int
	}{
		{50, 50},
		{-10, 0},
		{150, 100},
		{0, 0},
		{100, 100},
	}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().
				On(getScript, "42,false", nil).
				On("osascript -e set volume output volume "+itoa(c.sent), "", nil)
			if _, err := sound.New(f).Set(context.Background(), c.in); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, call := range f.Calls() {
				if len(call.Args) == 2 && call.Args[1] == "set volume output volume "+itoa(c.sent) {
					found = true
				}
			}
			if !found {
				t.Fatalf("Set did not issue the expected osascript; calls=%+v", f.Calls())
			}
		})
	}
}

func TestAdjust_AppliesDelta(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On(getScript, "60,false", nil).
		On("osascript -e set volume output volume 70", "", nil)
	if _, err := sound.New(f).Adjust(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
}

func TestAdjust_GetError(t *testing.T) {
	t.Parallel()
	svc := sound.New(runner.NewFake().On(getScript, "", errors.New("get fail")))
	if _, err := svc.Adjust(context.Background(), 1); err == nil {
		t.Fatal("expected error from Get")
	}
}

func TestMax_SetsTo100(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("osascript -e set volume output volume 100", "", nil).
		On(getScript, "100,false", nil)
	st, err := sound.New(f).Max(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Level != 100 {
		t.Fatalf("level=%d", st.Level)
	}
}

func TestMuteUnmuteToggle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		fn       func(svc *sound.Service) (sound.State, error)
		initial  string
		wantArg  string
		wantAfter string
	}{
		{"Mute", func(s *sound.Service) (sound.State, error) { return s.Mute(context.Background()) }, "50,false", "set volume output muted true", "50,true"},
		{"Unmute", func(s *sound.Service) (sound.State, error) { return s.Unmute(context.Background()) }, "50,true", "set volume output muted false", "50,false"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().
				On(getScript, c.initial, nil).
				On("osascript -e "+c.wantArg, "", nil)
			// After setting, Get is called again — swap canned response.
			f.On(getScript, c.wantAfter, nil)
			if _, err := c.fn(sound.New(f)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestToggleMute_FromUnmuted(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On(getScript, "50,false", nil).
		On("osascript -e set volume output muted true", "", nil)
	if _, err := sound.New(f).ToggleMute(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestToggleMute_FromMuted(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On(getScript, "50,true", nil).
		On("osascript -e set volume output muted false", "", nil)
	if _, err := sound.New(f).ToggleMute(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestToggleMute_GetError(t *testing.T) {
	t.Parallel()
	svc := sound.New(runner.NewFake().On(getScript, "", errors.New("x")))
	if _, err := svc.ToggleMute(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSay_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("say hello world", "", nil)
	if err := sound.New(f).Say(context.Background(), "hello world"); err != nil {
		t.Fatal(err)
	}
}

func TestSay_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("say bad", "", errors.New("no voice"))
	if err := sound.New(f).Say(context.Background(), "bad"); err == nil {
		t.Fatal("expected error")
	}
}

// itoa avoids importing strconv in the test file for a handful of uses.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
