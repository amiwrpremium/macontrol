package battery_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestGet_ChargingLaptop(t *testing.T) {
	t.Parallel()
	out := `Now drawing from 'AC Power'
 -InternalBattery-0 (id=12345)	84%; charging; 1:02 remaining present: true
`
	f := runner.NewFake().On("pmset -g batt", out, nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent != 84 {
		t.Errorf("percent = %d", st.Percent)
	}
	if st.State != battery.StateCharging {
		t.Errorf("state = %s", st.State)
	}
	if st.TimeRemaining != "1:02 remaining" {
		t.Errorf("time = %q", st.TimeRemaining)
	}
}

func TestGet_Discharging(t *testing.T) {
	t.Parallel()
	out := ` -InternalBattery-0 (id=12345)	55%; discharging; 3:42 remaining present: true
`
	f := runner.NewFake().On("pmset -g batt", out, nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent != 55 || st.State != battery.StateDischarging {
		t.Fatalf("got %+v", st)
	}
}

func TestGet_Charged(t *testing.T) {
	t.Parallel()
	out := ` -InternalBattery-0 (id=12345)	100%; charged; 0:00 remaining present: true
`
	f := runner.NewFake().On("pmset -g batt", out, nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.State != battery.StateACFull {
		t.Errorf("state = %s", st.State)
	}
	if st.Percent != 100 {
		t.Errorf("percent = %d", st.Percent)
	}
}

func TestGet_NoBattery_NoBatteriesAvailable(t *testing.T) {
	t.Parallel()
	out := "Now drawing from 'AC Power'\nNo batteries available\n"
	f := runner.NewFake().On("pmset -g batt", out, nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Present {
		t.Error("expected Present=false")
	}
	if st.Percent != -1 {
		t.Errorf("percent = %d", st.Percent)
	}
	if st.State != battery.StateUnknown {
		t.Errorf("state = %s", st.State)
	}
}

func TestGet_NoBattery_NotPresent(t *testing.T) {
	t.Parallel()
	out := "Battery is not present\n"
	f := runner.NewFake().On("pmset -g batt", out, nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Present {
		t.Error("expected Present=false")
	}
}

func TestGet_UnparseableOutput(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pmset -g batt", "completely garbage\nlines\n", nil)
	if _, err := battery.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pmset -g batt", "", errors.New("pmset not found"))
	if _, err := battery.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet_NoTimeRemaining(t *testing.T) {
	t.Parallel()
	// Some pmset outputs omit the "; <time> remaining" third segment.
	out := ` -InternalBattery-0 (id=12345)	50%; discharging`
	f := runner.NewFake().On("pmset -g batt", out+"\n", nil)
	st, err := battery.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent != 50 || st.State != battery.StateDischarging {
		t.Errorf("got %+v", st)
	}
}

func TestGetHealth_FullData(t *testing.T) {
	t.Parallel()
	out := `    Battery Information:
      Model Information:
          Manufacturer: SMP
      Charge Information:
          Cycle Count: 123
      Health Information:
          Condition: Normal
          Maximum Capacity: 91%
      AC Charger Information:
          Wattage (W): 70
`
	f := runner.NewFake().On("system_profiler SPPowerDataType", out, nil)
	h, err := battery.New(f).GetHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.CycleCount != 123 || h.Condition != "Normal" || h.MaxCapacity != "91%" || h.ChargerWattage != "70W" {
		t.Fatalf("unexpected health: %+v", h)
	}
}

func TestGetHealth_PartialData(t *testing.T) {
	t.Parallel()
	out := `          Cycle Count: 50
`
	f := runner.NewFake().On("system_profiler SPPowerDataType", out, nil)
	h, err := battery.New(f).GetHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.CycleCount != 50 || h.Condition != "" || h.MaxCapacity != "" {
		t.Fatalf("unexpected: %+v", h)
	}
}

func TestGetHealth_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("system_profiler SPPowerDataType", "", errors.New("fail"))
	if _, err := battery.New(f).GetHealth(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
