package bluetooth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

func TestGet_On(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil -p", "1\n", nil)
	st, err := bluetooth.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.PowerOn {
		t.Fatal("expected PowerOn=true")
	}
}

func TestGet_Off(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil -p", "0\n", nil)
	st, err := bluetooth.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.PowerOn {
		t.Fatal("expected PowerOn=false")
	}
}

func TestGet_NotInstalled(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil -p", "", errors.New("blueutil not found"))
	if _, err := bluetooth.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetPower_Both(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   bool
		cmd  string
	}{
		{"on", true, "blueutil --power 1"},
		{"off", false, "blueutil --power 0"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().On(c.cmd, "", nil)
			st, err := bluetooth.New(f).SetPower(context.Background(), c.in)
			if err != nil {
				t.Fatal(err)
			}
			if st.PowerOn != c.in {
				t.Fatalf("got %+v", st)
			}
		})
	}
}

func TestSetPower_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --power 1", "", errors.New("fail"))
	if _, err := bluetooth.New(f).SetPower(context.Background(), true); err == nil {
		t.Fatal("expected error")
	}
}

func TestToggle_FromOff(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("blueutil -p", "0\n", nil).
		On("blueutil --power 1", "", nil)
	st, err := bluetooth.New(f).Toggle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.PowerOn {
		t.Fatal("expected flipped to On")
	}
}

func TestToggle_FromOn(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("blueutil -p", "1\n", nil).
		On("blueutil --power 0", "", nil)
	st, err := bluetooth.New(f).Toggle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.PowerOn {
		t.Fatal("expected flipped to Off")
	}
}

func TestToggle_GetError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil -p", "", errors.New("fail"))
	if _, err := bluetooth.New(f).Toggle(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestPaired_ParsesJSON(t *testing.T) {
	t.Parallel()
	out := `[{"address":"aa-bb-cc-dd-ee-ff","name":"AirPods","connected":true,"paired":true,"favourite":true},
{"address":"11-22-33-44-55-66","name":"Keyboard","connected":false,"paired":true,"favourite":false}]`
	f := runner.NewFake().On("blueutil --paired --format json", out, nil)
	devs, err := bluetooth.New(f).Paired(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 2 {
		t.Fatalf("count = %d", len(devs))
	}
	if devs[0].Name != "AirPods" || !devs[0].Connected {
		t.Fatalf("unexpected: %+v", devs[0])
	}
}

func TestPaired_MalformedJSON(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --paired --format json", "not json", nil)
	if _, err := bluetooth.New(f).Paired(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestPaired_Empty(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --paired --format json", "[]", nil)
	devs, err := bluetooth.New(f).Paired(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected empty, got %d", len(devs))
	}
}

func TestConnected_ParsesJSON(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --connected --format json",
		`[{"address":"aa-bb-cc-dd-ee-ff","name":"AirPods","connected":true,"paired":true,"favourite":true}]`,
		nil)
	devs, err := bluetooth.New(f).Connected(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 {
		t.Fatalf("count = %d", len(devs))
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --connect aa-bb-cc-dd-ee-ff", "", nil)
	if err := bluetooth.New(f).Connect(context.Background(), "aa-bb-cc-dd-ee-ff"); err != nil {
		t.Fatal(err)
	}
}

func TestConnect_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --connect xx", "", errors.New("not paired"))
	if err := bluetooth.New(f).Connect(context.Background(), "xx"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDisconnect(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("blueutil --disconnect aa-bb-cc-dd-ee-ff", "", nil)
	if err := bluetooth.New(f).Disconnect(context.Background(), "aa-bb-cc-dd-ee-ff"); err != nil {
		t.Fatal(err)
	}
}
