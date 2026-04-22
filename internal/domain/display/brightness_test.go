package display_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestGet_SingleDisplay(t *testing.T) {
	t.Parallel()
	out := "display 0: brightness 0.682354\n"
	f := runner.NewFake().On("brightness -l", out, nil)
	st, err := display.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Level < 0.68 || st.Level > 0.69 {
		t.Fatalf("level=%f", st.Level)
	}
}

func TestGet_MultipleDisplays_FirstWins(t *testing.T) {
	t.Parallel()
	out := "display 1: brightness 0.1\ndisplay 0: brightness 0.5\ndisplay 2: brightness 0.9\n"
	f := runner.NewFake().On("brightness -l", out, nil)
	st, err := display.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Level != 0.1 {
		t.Fatalf("level=%f want 0.1 (first matching line wins)", st.Level)
	}
}

func TestGet_Clamps01(t *testing.T) {
	t.Parallel()
	out := "display 0: brightness 1.5\n"
	f := runner.NewFake().On("brightness -l", out, nil)
	st, _ := display.New(f).Get(context.Background())
	if st.Level != 1.0 {
		t.Errorf("over-range not clamped; got %f", st.Level)
	}
	out2 := "display 0: brightness -0.5\n"
	f2 := runner.NewFake().On("brightness -l", out2, nil)
	st2, _ := display.New(f2).Get(context.Background())
	if st2.Level != 0.0 {
		t.Errorf("under-range not clamped; got %f", st2.Level)
	}
}

func TestGet_BrewMissing(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("brightness -l", "", errors.New("brightness not installed"))
	st, err := display.New(f).Get(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if st.Level != -1 {
		t.Errorf("level=%f want -1", st.Level)
	}
}

// TestGet_HeaderAndErrorOnly mirrors the live Mac output on macOS 15+
// where CoreDisplay denies the brightness CLI's private-API call. The
// tool exits 0 but only emits the header line and an error line; no
// `display N: brightness <float>` line ever appears. Old parser used
// to grab the header line as a value and crash with a parse error;
// new parser must surface the tool's own error line.
func TestGet_HeaderAndErrorOnly(t *testing.T) {
	t.Parallel()
	out := "display 0: main, active, awake, online, built-in, ID 0x1\n" +
		"brightness: failed to get brightness of display 0x1 (error -536870201)\n"
	f := runner.NewFake().On("brightness -l", out, nil)
	st, err := display.New(f).Get(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if st.Level != -1 {
		t.Errorf("level=%f want -1", st.Level)
	}
	if !strings.Contains(err.Error(), "error -536870201") {
		t.Errorf("error should surface tool's own message, got: %v", err)
	}
}

func TestGet_UnparseableLine(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("brightness -l", "display 0: brightness notanumber\n", nil)
	if _, err := display.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGet_ShortLine(t *testing.T) {
	t.Parallel()
	// display 0 line exists but is truncated before the value field.
	f := runner.NewFake().On("brightness -l", "display 0: brightness\n", nil)
	if _, err := display.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error on short line")
	}
}

func TestSet_ClampAndFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, sent string
		level    float64
	}{
		{"in-range", "0.500", 0.5},
		{"negative clamps", "0.000", -0.5},
		{"over clamps", "1.000", 1.5},
		{"zero", "0.000", 0.0},
		{"one", "1.000", 1.0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().On("brightness "+c.sent, "", nil)
			st, err := display.New(f).Set(context.Background(), c.level)
			if err != nil {
				t.Fatal(err)
			}
			if st.Level < 0 || st.Level > 1 {
				t.Errorf("returned level outside [0,1]: %f", st.Level)
			}
		})
	}
}

func TestSet_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("brightness 0.500", "", errors.New("permission denied"))
	st, err := display.New(f).Set(context.Background(), 0.5)
	if err == nil {
		t.Fatal("expected error")
	}
	if st.Level != -1 {
		t.Fatalf("level=%f", st.Level)
	}
}

func TestAdjust_GetFailure(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("brightness -l", "", errors.New("no brew"))
	if _, err := display.New(f).Adjust(context.Background(), 0.1); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdjust_AppliesDelta(t *testing.T) {
	t.Parallel()
	// Set registers both the exact IEEE-754 sum 0.400+0.2=0.60000... and the
	// rounded 3-decimal form 0.600, depending on formatFloat's output.
	f := runner.NewFake().
		On("brightness -l", "display 0: brightness 0.400\n", nil).
		On("brightness 0.600", "", nil).
		On("brightness 0.600000", "", nil)
	st, err := display.New(f).Adjust(context.Background(), 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if diff := st.Level - 0.6; diff > 0.01 || diff < -0.01 {
		t.Fatalf("level=%f; want ~0.6", st.Level)
	}
}

func TestScreensaver(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("open -a ScreenSaverEngine", "", nil)
	if err := display.New(f).Screensaver(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestScreensaver_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("open -a ScreenSaverEngine", "", errors.New("failed"))
	if err := display.New(f).Screensaver(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
