package flows_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/domain/media"
	"github.com/amiwrpremium/macontrol/internal/domain/notify"
	"github.com/amiwrpremium/macontrol/internal/domain/power"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// -------------------------------- setbrightness --------------------------------

func TestSetBrightnessFlow_Name(t *testing.T) {
	t.Parallel()
	if flows.NewSetBrightness(display.New(runner.NewFake())).Name() != "dsp:set" {
		t.Fatal("wrong name")
	}
}

func TestSetBrightnessFlow_Valid(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("brightness 0.750", "", nil)
	flow := flows.NewSetBrightness(display.New(f))
	r := flow.Handle(context.Background(), "75")
	if !r.Done {
		t.Fatal("expected Done")
	}
	if !strings.Contains(r.Text, "75") {
		t.Errorf("text = %q", r.Text)
	}
}

func TestSetBrightnessFlow_Invalid(t *testing.T) {
	t.Parallel()
	flow := flows.NewSetBrightness(display.New(runner.NewFake()))
	cases := []string{"abc", "-1", "101", ""}
	for _, in := range cases {
		r := flow.Handle(context.Background(), in)
		if r.Done {
			t.Errorf("input %q should not terminate flow", in)
		}
	}
}

func TestSetBrightnessFlow_StartPrompt(t *testing.T) {
	t.Parallel()
	flow := flows.NewSetBrightness(display.New(runner.NewFake()))
	r := flow.Start(context.Background())
	if !strings.Contains(r.Text, "0") || !strings.Contains(r.Text, "100") {
		t.Errorf("prompt = %q", r.Text)
	}
}

// -------------------------------- keepawake --------------------------------

func TestKeepAwakeFlow_Valid(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sh -c nohup caffeinate -d -t 300 >/dev/null 2>&1 &", "", nil)
	flow := flows.NewKeepAwake(power.New(f))
	r := flow.Handle(context.Background(), "5")
	if !r.Done {
		t.Fatal("expected Done")
	}
}

func TestKeepAwakeFlow_Invalid(t *testing.T) {
	t.Parallel()
	flow := flows.NewKeepAwake(power.New(runner.NewFake()))
	for _, in := range []string{"abc", "0", "-1", "1441", "1000000"} {
		r := flow.Handle(context.Background(), in)
		if r.Done {
			t.Errorf("input %q should not terminate", in)
		}
	}
}

func TestKeepAwakeFlow_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		"sh -c nohup caffeinate -d -t 60 >/dev/null 2>&1 &", "", errors.New("sh not found"))
	flow := flows.NewKeepAwake(power.New(f))
	r := flow.Handle(context.Background(), "1")
	if !r.Done {
		t.Fatal("expected Done on error (flow gives up)")
	}
	if !strings.Contains(r.Text, "could not") {
		t.Errorf("text = %q", r.Text)
	}
}

func TestKeepAwakeFlow_Name(t *testing.T) {
	t.Parallel()
	if flows.NewKeepAwake(power.New(runner.NewFake())).Name() != "pwr:keepawake" {
		t.Fatal()
	}
}

// -------------------------------- joinwifi (multi-step) --------------------------------

func newWifiFake() *runner.Fake {
	return runner.NewFake().
		On("networksetup -listallhardwareports",
			"Hardware Port: Wi-Fi\nDevice: en0\n", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
}

func TestJoinWifiFlow_TwoStep(t *testing.T) {
	t.Parallel()
	f := newWifiFake().
		On("networksetup -setairportnetwork en0 home secret", "", nil)
	flow := flows.NewJoinWifi(wifi.New(f))

	first := flow.Handle(context.Background(), "home")
	if first.Done {
		t.Fatal("first step should not be Done")
	}
	if !strings.Contains(first.Text, "password") {
		t.Errorf("expected password prompt: %q", first.Text)
	}
	second := flow.Handle(context.Background(), "secret")
	if !second.Done {
		t.Fatal("second step should be Done")
	}
	if !strings.Contains(second.Text, "Joined") {
		t.Errorf("text = %q", second.Text)
	}
}

func TestJoinWifiFlow_EmptySSID(t *testing.T) {
	t.Parallel()
	flow := flows.NewJoinWifi(wifi.New(newWifiFake()))
	r := flow.Handle(context.Background(), "")
	if r.Done {
		t.Fatal("empty SSID should re-prompt, not finish")
	}
}

func TestJoinWifiFlow_OpenNetwork(t *testing.T) {
	t.Parallel()
	f := newWifiFake().
		On("networksetup -setairportnetwork en0 open", "", nil)
	flow := flows.NewJoinWifi(wifi.New(f))
	_ = flow.Handle(context.Background(), "open")
	r := flow.Handle(context.Background(), "-")
	if !r.Done {
		t.Fatal("expected Done")
	}
}

func TestJoinWifiFlow_JoinFailure(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", "Hardware Port: Wi-Fi\nDevice: en0\n", nil).
		On("networksetup -setairportnetwork en0 bad pwd", "", errors.New("Bad password"))
	flow := flows.NewJoinWifi(wifi.New(f))
	_ = flow.Handle(context.Background(), "bad")
	r := flow.Handle(context.Background(), "pwd")
	if !r.Done {
		t.Fatal("expected Done on failure")
	}
	if !strings.Contains(r.Text, "could not join") {
		t.Errorf("text = %q", r.Text)
	}
}

// -------------------------------- killproc --------------------------------

func TestKillProcFlow_ByPID(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill 999", "", nil)
	flow := flows.NewKillProc(system.New(f))
	r := flow.Handle(context.Background(), "999")
	if !r.Done {
		t.Fatal()
	}
}

func TestKillProcFlow_ByName(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("killall Safari", "", nil)
	flow := flows.NewKillProc(system.New(f))
	r := flow.Handle(context.Background(), "Safari")
	if !r.Done {
		t.Fatal()
	}
}

func TestKillProcFlow_Empty(t *testing.T) {
	t.Parallel()
	flow := flows.NewKillProc(system.New(runner.NewFake()))
	r := flow.Handle(context.Background(), "   ")
	if r.Done {
		t.Fatal("empty input should re-prompt")
	}
}

func TestKillProcFlow_PIDFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill 1", "", errors.New("No such process"))
	flow := flows.NewKillProc(system.New(f))
	r := flow.Handle(context.Background(), "1")
	if !r.Done {
		t.Fatal()
	}
	if !strings.Contains(r.Text, "failed") {
		t.Errorf("text = %q", r.Text)
	}
}

func TestKillProcFlow_NameFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("killall nope", "", errors.New("no such"))
	flow := flows.NewKillProc(system.New(f))
	r := flow.Handle(context.Background(), "nope")
	if !r.Done {
		t.Fatal()
	}
}

// -------------------------------- notify / say --------------------------------

func TestSendNotifyFlow_TitleAndBody(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // force osascript path
	f := runner.NewFake().On(
		`osascript -e display notification "body" with title "title"`, "", nil)
	flow := flows.NewSendNotify(notify.New(f))
	r := flow.Handle(context.Background(), "title | body")
	if !r.Done {
		t.Fatal()
	}
}

func TestSendNotifyFlow_BodyOnly(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	f := runner.NewFake().On(`osascript -e display notification "just body"`, "", nil)
	flow := flows.NewSendNotify(notify.New(f))
	r := flow.Handle(context.Background(), "just body")
	if !r.Done {
		t.Fatal()
	}
}

func TestSendNotifyFlow_Empty(t *testing.T) {
	t.Parallel()
	flow := flows.NewSendNotify(notify.New(runner.NewFake()))
	r := flow.Handle(context.Background(), "  ")
	if r.Done {
		t.Fatal("empty should re-prompt")
	}
}

func TestSendNotifyFlow_Failure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	f := runner.NewFake().On(
		`osascript -e display notification "b" with title "t"`, "", errors.New("x"))
	flow := flows.NewSendNotify(notify.New(f))
	r := flow.Handle(context.Background(), "t | b")
	if !r.Done {
		t.Fatal()
	}
	if !strings.Contains(r.Text, "failed") {
		t.Errorf("text = %q", r.Text)
	}
}

func TestSayFlow_Valid(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("say hello", "", nil)
	flow := flows.NewSay(notify.New(f))
	r := flow.Handle(context.Background(), "hello")
	if !r.Done {
		t.Fatal()
	}
}

func TestSayFlow_Empty(t *testing.T) {
	t.Parallel()
	flow := flows.NewSay(notify.New(runner.NewFake()))
	r := flow.Handle(context.Background(), "   ")
	if r.Done {
		t.Fatal("empty should re-prompt")
	}
}

func TestSayFlow_Failure(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("say x", "", errors.New("bad voice"))
	flow := flows.NewSay(notify.New(f))
	r := flow.Handle(context.Background(), "x")
	if !r.Done {
		t.Fatal()
	}
}

// -------------------------------- clipset --------------------------------

func TestClipSetFlow_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e set the clipboard to "hello"`, "", nil)
	flow := flows.NewClipSet(tools.New(f))
	r := flow.Handle(context.Background(), "hello")
	if !r.Done {
		t.Fatal()
	}
}

func TestClipSetFlow_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e set the clipboard to "x"`, "", errors.New("x"))
	flow := flows.NewClipSet(tools.New(f))
	r := flow.Handle(context.Background(), "x")
	if !r.Done {
		t.Fatal()
	}
}

// -------------------------------- timezone --------------------------------

func TestTimezoneFlow_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("systemsetup -settimezone Europe/Istanbul", "", nil).
		On("systemsetup -gettimezone", "Time Zone: Europe/Istanbul\n", nil)
	flow := flows.NewTimezone(tools.New(f))
	r := flow.Handle(context.Background(), "Europe/Istanbul")
	if !r.Done {
		t.Fatal()
	}
}

func TestTimezoneFlow_Empty(t *testing.T) {
	t.Parallel()
	flow := flows.NewTimezone(tools.New(runner.NewFake()))
	r := flow.Handle(context.Background(), "  ")
	if r.Done {
		t.Fatal("empty should re-prompt")
	}
}

func TestTimezoneFlow_SetError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -settimezone Bad", "", errors.New("unknown tz"))
	flow := flows.NewTimezone(tools.New(f))
	r := flow.Handle(context.Background(), "Bad")
	if !r.Done {
		t.Fatal()
	}
}

// -------------------------------- shortcut --------------------------------

func TestShortcutFlow_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts run My Shortcut", "", nil)
	flow := flows.NewShortcut(tools.New(f))
	r := flow.Handle(context.Background(), "My Shortcut")
	if !r.Done {
		t.Fatal()
	}
}

func TestShortcutFlow_Empty(t *testing.T) {
	t.Parallel()
	flow := flows.NewShortcut(tools.New(runner.NewFake()))
	r := flow.Handle(context.Background(), "")
	if r.Done {
		t.Fatal("empty should re-prompt")
	}
}

func TestShortcutFlow_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts run x", "", errors.New("no"))
	flow := flows.NewShortcut(tools.New(f))
	r := flow.Handle(context.Background(), "x")
	if !r.Done {
		t.Fatal()
	}
}

// -------------------------------- record --------------------------------

func TestRecordFlow_Valid(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", nil)
	sent := false
	sender := func(_ context.Context, _ string) error {
		sent = true
		return nil
	}
	flow := flows.NewRecord(media.New(f), 1, sender)
	r := flow.Handle(context.Background(), "5")
	if !r.Done {
		t.Fatal()
	}
	if !sent {
		t.Fatal("sender was not invoked")
	}
}

func TestRecordFlow_InvalidDuration(t *testing.T) {
	t.Parallel()
	flow := flows.NewRecord(media.New(runner.NewFake()), 1, nil)
	for _, in := range []string{"0", "-1", "121", "abc"} {
		r := flow.Handle(context.Background(), in)
		if r.Done {
			t.Errorf("input %q should not terminate", in)
		}
	}
}

func TestRecordFlow_RecordFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", errors.New("TCC denied"))
	flow := flows.NewRecord(media.New(f), 1, nil)
	r := flow.Handle(context.Background(), "3")
	if !r.Done {
		t.Fatal()
	}
	if !strings.Contains(r.Text, "record failed") {
		t.Errorf("text = %q", r.Text)
	}
}

func TestRecordFlow_UploadFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("screencapture ", "", nil)
	bad := func(_ context.Context, _ string) error { return errors.New("telegram too big") }
	flow := flows.NewRecord(media.New(f), 1, bad)
	r := flow.Handle(context.Background(), "3")
	if !r.Done {
		t.Fatal()
	}
	if !strings.Contains(r.Text, "upload failed") {
		t.Errorf("text = %q", r.Text)
	}
}

// -------------------------------- registry extensions --------------------------------

type fakeFlow struct{ name string }

func (f fakeFlow) Name() string                         { return f.name }
func (f fakeFlow) Start(context.Context) flows.Response { return flows.Response{} }
func (fakeFlow) Handle(context.Context, string) flows.Response {
	return flows.Response{Done: true}
}

func TestRegistry_InstallReplacesExisting(t *testing.T) {
	t.Parallel()
	r := flows.NewRegistry(time.Minute)
	r.Install(1, fakeFlow{name: "a"})
	r.Install(1, fakeFlow{name: "b"})
	got, ok := r.Active(1)
	if !ok {
		t.Fatal("expected active flow")
	}
	if got.Name() != "b" {
		t.Fatalf("expected replaced flow, got %q", got.Name())
	}
}

func TestRegistry_FinishAliasesCancel(t *testing.T) {
	t.Parallel()
	r := flows.NewRegistry(time.Minute)
	r.Install(7, fakeFlow{name: "x"})
	r.Finish(7)
	if _, ok := r.Active(7); ok {
		t.Fatal("expected flow gone after Finish")
	}
}

func TestRegistry_StartJanitor_StopsOnCtxCancel(t *testing.T) {
	t.Parallel()
	r := flows.NewRegistry(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	r.StartJanitor(ctx)
	cancel()
	// Janitor should exit without panicking; sanity-check after brief wait.
	time.Sleep(50 * time.Millisecond)
}
