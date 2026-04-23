package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
	"github.com/amiwrpremium/macontrol/internal/telegram/telegramtest"
)

// ============================ nav ============================

func TestNav_Home(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "nav:home")); err != nil {
		t.Fatal(err)
	}
	if len(h.Recorder.ByMethod("editMessageText")) != 1 {
		t.Fatal("expected edit to home")
	}
}

func TestNav_Unknown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "nav:nope"))
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

// ============================ dsp ============================

func TestDsp_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("brightness -l", "display 0: brightness 0.700\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:open")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "70%") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestDsp_OpenWithoutBrew(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("brightness -l", "", errors.New("not installed"))
	// Handler swallows and shows a keyboard anyway.
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:open")); err != nil {
		t.Fatal(err)
	}
	if len(h.Recorder.ByMethod("editMessageText")) != 1 {
		t.Fatal("expected an edit even when Get fails")
	}
}

func TestDsp_Up(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("brightness -l", "display 0: brightness 0.500\n", nil).
		On("brightness 0.550", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:up:5")); err != nil {
		t.Fatal(err)
	}
}

func TestDsp_Down(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("brightness -l", "display 0: brightness 0.500\n", nil).
		On("brightness 0.400", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:down:10")); err != nil {
		t.Fatal(err)
	}
}

func TestDsp_Set_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:set")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow installed")
	}
}

func TestDsp_Screensaver(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("open -a ScreenSaverEngine", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:screensaver")); err != nil {
		t.Fatal(err)
	}
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

func TestDsp_Unknown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "dsp:nope"))
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

// ============================ pwr ============================

func TestPwr_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:open")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_Lock(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pmset displaysleepnow", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:lock")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_Sleep(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pmset sleepnow", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:sleep")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_RestartShowsConfirm(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:restart")); err != nil {
		t.Fatal(err)
	}
	edits := h.Recorder.ByMethod("editMessageText")
	if len(edits) != 1 {
		t.Fatal("expected an edit")
	}
	if !strings.Contains(edits[0].Fields["text"], "Confirm") {
		t.Errorf("expected Confirm text; got %q", edits[0].Fields["text"])
	}
}

func TestPwr_RestartConfirmed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(`osascript -e tell application "System Events" to restart`, "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:restart:ok")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_ShutdownConfirmed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(`osascript -e tell application "System Events" to shut down`, "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:shutdown:ok")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_LogoutConfirmed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(`osascript -e tell application "System Events" to log out`, "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:logout:ok")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_KeepAwake_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:keepawake")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

func TestPwr_CancelAwake(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pkill -x caffeinate", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:cancelawake")); err != nil {
		t.Fatal(err)
	}
}

func TestPwr_Unknown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "pwr:nope"))
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

// ============================ bat ============================

func TestBat_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pmset -g batt", " -InternalBattery-0 (id=1)	80%; charging; 1:00 remaining present: true\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bat:open")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "80%") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestBat_Health(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("system_profiler SPPowerDataType", "Cycle Count: 100\nCondition: Normal\n", nil).
		On("pmset -g batt", " -InternalBattery-0 (id=1)	80%; charging\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bat:health")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "Normal") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestBat_OpenError(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pmset -g batt", "", errors.New("fail"))
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bat:open"))
	if !strings.Contains(h.Recorder.Last().Fields["text"], "unavailable") {
		t.Errorf("expected unavailable: %q", h.Recorder.Last().Fields["text"])
	}
}

func TestBat_Unknown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bat:nope"))
}

// ============================ wif ============================

func wifiRules(h *harness) {
	h.Fake.
		On("networksetup -listallhardwareports", "Hardware Port: Wi-Fi\nDevice: en0\n", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
}

func TestWif_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	wifiRules(h)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:open")); err != nil {
		t.Fatal(err)
	}
}

func TestWif_Toggle(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	wifiRules(h)
	h.Fake.On("networksetup -setairportpower en0 off", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:toggle")); err != nil {
		t.Fatal(err)
	}
}

func TestWif_Info(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	wifiRules(h)
	h.Fake.On("wdutil info", "SSID: home\nBSSID: xx\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:info")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "diagnostics") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestWif_DNSPresets(t *testing.T) {
	for _, preset := range []string{"cf", "google", "reset"} {
		preset := preset
		t.Run(preset, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			wifiRules(h)
			// Any of these DNS calls will be picked up.
			h.Fake.
				On("networksetup -setdnsservers Wi-Fi 1.1.1.1 1.0.0.1", "", nil).
				On("networksetup -setdnsservers Wi-Fi 8.8.8.8 8.8.4.4", "", nil).
				On("networksetup -setdnsservers Wi-Fi Empty", "", nil)
			if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
				newCallbackUpdate("id", "wif:dns:"+preset)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWif_Speedtest(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	wifiRules(h)
	h.Fake.On("networkQuality -v", "Downlink capacity: 100 Mbps\nUplink capacity: 50 Mbps\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:speedtest")); err != nil {
		t.Fatal(err)
	}
}

func TestWif_SpeedtestDisabled(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Deps.Capability.Features.NetworkQuality = false
	// No fake rules needed — handler should short-circuit.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:speedtest"))
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

func TestWif_Join_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "wif:join")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

// ============================ bt ============================

func TestBt_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("blueutil -p", "1\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bt:open")); err != nil {
		t.Fatal(err)
	}
}

func TestBt_Toggle(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("blueutil -p", "0\n", nil).
		On("blueutil --power 1", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bt:toggle")); err != nil {
		t.Fatal(err)
	}
}

func TestBt_PairedPopulatesShortmap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("blueutil --paired --format json",
		`[{"address":"aa-bb-cc","name":"AirPods","connected":true,"paired":true}]`, nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bt:paired")); err != nil {
		t.Fatal(err)
	}
	if h.Deps.ShortMap.Size() != 1 {
		t.Errorf("expected 1 shortmap entry, got %d", h.Deps.ShortMap.Size())
	}
}

func TestBt_ConnectExpiredShortID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bt:conn:notreal"))
	last := h.Recorder.Last()
	if last.Method != "answerCallbackQuery" {
		t.Fatalf("expected toast; got %s", last.Method)
	}
	if !strings.Contains(last.Fields["text"], "expired") {
		t.Errorf("toast = %q", last.Fields["text"])
	}
}

func TestBt_ConnectNoArgs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "bt:conn"))
	if h.Recorder.Last().Method != "answerCallbackQuery" {
		t.Fatal("expected toast")
	}
}

// ============================ sys ============================

func TestSys_Menu(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:open")); err != nil {
		t.Fatal(err)
	}
}

func TestSys_Info(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("sw_vers", "ProductName: macOS\nProductVersion: 15.3\n", nil).
		On("hostname", "tower\n", nil).
		On("sysctl -n hw.model", "MacBookPro\n", nil).
		On("sysctl -n machdep.cpu.brand_string", "Apple M3\n", nil).
		On("sysctl -n hw.memsize", "34359738368\n", nil).
		On("uptime", "21:44  up 3 days,  6:27, 1 user, load averages: 4.97 4.57 4.19\n", nil).
		On("system_profiler SPHardwareDataType", "Total Number of Cores: 12\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:info")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.Last().Fields["text"]
	for _, want := range []string{"macOS 15.3", "Uptime:", "3 days", "6h 27m", "Logged-in user", "Load avg", "4.97", "12 cores"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
		}
	}
}

func TestSys_Temp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("powermetrics -n 1 -i 1000 --samplers thermal", "Current pressure level: Nominal\n", nil).
		On("smctemp -c", "", errors.New("missing")).
		On("smctemp -g", "", errors.New("missing"))
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:temp")); err != nil {
		t.Fatal(err)
	}
}

func TestSys_Mem(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("top -l 1 -s 0",
			"Processes: 500\nPhysMem: 23G used (3401M wired, 8367M compressor), 550M unused.\n", nil).
		On("memory_pressure",
			"The system has 25769803776 (1572864 pages with a page size of 16384).\n"+
				"System-wide memory free percentage: 18%\n", nil).
		On("sysctl vm.swapusage",
			"vm.swapusage: total = 2048.00M  used = 1234.56M  free = 813.44M  (encrypted)\n", nil).
		On("ps -Ao pid,pcpu,pmem,comm -m",
			"  PID  %CPU %MEM COMM\n"+
				"  100  10.5 12.4 /Applications/Chrome\n"+
				"  101   3.1  8.7 /Applications/Slack\n"+
				"  102   1.0  5.1 WindowServer\n", nil).
		// info.Info() runs alongside Memory() to grab TotalRAMBytes.
		On("sw_vers", "ProductName: macOS\nProductVersion: 26.0\n", nil).
		On("hostname", "tower\n", nil).
		On("sysctl -n hw.model", "Mac16,8\n", nil).
		On("sysctl -n machdep.cpu.brand_string", "Apple M4 Pro\n", nil).
		On("sysctl -n hw.memsize", "25769803776\n", nil).
		On("uptime", "21:44  up 3 days,  6:27, 1 user, load averages: 4.97 4.57 4.19\n", nil).
		On("system_profiler SPHardwareDataType", "Total Number of Cores: 12\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:mem")); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	text := last.Fields["text"]
	for _, want := range []string{"Used:", "24.0 GiB", "(95%)", "Wired:", "Compressed:", "Swap used:", "Pressure:", "Warning", "Top by RAM"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
		}
	}
	// Top-3 RAM hogs are now per-process inline buttons routing to sys:proc:<pid>.
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	for _, wantCB := range []string{"sys:proc:100", "sys:proc:101", "sys:proc:102"} {
		found := false
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				if btn.CallbackData == wantCB {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("missing per-process button %q", wantCB)
		}
	}
}

func TestSys_CPU(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("uptime", "21:46  up 3 days,  6:29, 1 user, load averages: 5.41 4.92 4.39\n", nil).
		On("top -l 1 -s 0",
			"Processes: 500\nCPU usage: 20.85% user, 16.25% sys, 62.88% idle\n", nil).
		On("ps -Ao pid,pcpu,pmem,comm -r",
			"  PID  %CPU %MEM COMM\n"+
				"  100 12.4 1.0 /Applications/Chrome\n"+
				"  101  8.7 0.5 some-process\n"+
				"  102  5.1 0.2 WindowServer\n", nil).
		// CPU panel also calls Info() to get CPUCores for per-core %.
		On("sw_vers", "ProductName: macOS\nProductVersion: 26.0\n", nil).
		On("hostname", "tower\n", nil).
		On("sysctl -n hw.model", "Mac16,8\n", nil).
		On("sysctl -n machdep.cpu.brand_string", "Apple M4 Pro\n", nil).
		On("sysctl -n hw.memsize", "25769803776\n", nil).
		On("system_profiler SPHardwareDataType", "Total Number of Cores: 12\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:cpu")); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	text := last.Fields["text"]
	for _, want := range []string{"Busy:", "37%", "User", "Kernel", "Idle", "Load avg", "5.41", "12 cores", "Top by CPU"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
		}
	}
	// Top-3 CPU hogs are now per-process inline buttons routing to sys:proc:<pid>.
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	for _, wantCB := range []string{"sys:proc:100", "sys:proc:101", "sys:proc:102"} {
		found := false
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				if btn.CallbackData == wantCB {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("missing per-process button %q", wantCB)
		}
	}
}

func TestSys_Top(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("ps -Ao pid,pcpu,pmem,comm -r",
		"  PID  %CPU %MEM COMM\n"+
			"  100  10.0 5.0 /Applications/App.app/Contents/MacOS/App\n"+
			"  200  20.0 1.0 /usr/bin/foo\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:top")); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	text := last.Fields["text"]
	if !strings.Contains(text, "Top 10 by CPU") || !strings.Contains(text, "Tap a process") {
		t.Errorf("text = %q", text)
	}
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	// Per-process buttons should encode sys:proc:<pid>.
	wantPIDs := map[string]bool{"sys:proc:100": false, "sys:proc:200": false}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if _, want := wantPIDs[btn.CallbackData]; want {
				wantPIDs[btn.CallbackData] = true
			}
		}
	}
	for cb, found := range wantPIDs {
		if !found {
			t.Errorf("missing per-process button %q", cb)
		}
	}
}

func TestSys_Proc_DrillDown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("ps -Ao pid,pcpu,pmem,comm -r",
		"  PID  %CPU %MEM COMM\n"+
			"  100  10.0 5.0 /Applications/App.app/Contents/MacOS/App\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:proc:100")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.Last().Fields["text"]
	for _, want := range []string{"App", "PID:", "100", "10.0%", "5.0%", "/Applications/App.app"} {
		if !strings.Contains(text, want) {
			t.Errorf("drill-down missing %q; got %q", want, text)
		}
	}
}

func TestSys_KillPID_SendsSIGTERM(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("kill 100", "", nil).
		On("ps -Ao pid,pcpu,pmem,comm -r",
			"  PID  %CPU %MEM COMM\n  200  5.0 1.0 /usr/bin/foo\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:kill-pid:100")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "SIGTERM sent to PID") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestSys_Kill9_RequiresConfirm(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("ps -Ao pid,pcpu,pmem,comm -r",
		"  PID  %CPU %MEM COMM\n  100  10.0 5.0 /Apps/App\n", nil)

	// First tap — no confirmation argument; should render the confirm page,
	// NOT call kill -9.
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id1", "sys:kill9:100")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "Force kill PID 100") {
		t.Errorf("expected confirm page; got %q", h.Recorder.Last().Fields["text"])
	}

	// Second tap with the "ok" confirmation arg — must invoke kill -9.
	h.Fake.On("kill -9 100", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id2", "sys:kill9:100:ok")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "SIGKILL sent to PID") {
		t.Errorf("text = %q", h.Recorder.Last().Fields["text"])
	}
}

func TestSys_Kill_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:kill")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

// ============================ med ============================

func TestMed_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "med:open")); err != nil {
		t.Fatal(err)
	}
}

func TestMed_Shot_Failure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("screencapture ", "", errors.New("TCC denied"))
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "med:shot")); err != nil {
		t.Fatal(err)
	}
	// When the screencapture fails, handler sends a text message with the error.
	if len(h.Recorder.ByMethod("sendMessage")) == 0 {
		t.Fatal("expected sendMessage on failure")
	}
}

func TestMed_Photo_Failure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("imagesnap ", "", errors.New("no camera"))
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "med:photo")); err != nil {
		t.Fatal(err)
	}
}

func TestMed_Record_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "med:record")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

// ============================ ntf ============================

func TestNtf_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "ntf:open")); err != nil {
		t.Fatal(err)
	}
}

func TestNtf_Send_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "ntf:send")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

func TestNtf_Say_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "ntf:say")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

// ============================ tls ============================

func TestTls_Open(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:open")); err != nil {
		t.Fatal(err)
	}
}

func TestTls_ClipGet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("pbpaste", "hello", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:clip:get")); err != nil {
		t.Fatal(err)
	}
}

func TestTls_ClipSet_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:clip:set")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
	}
}

func TestTls_Tz_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:tz")); err != nil {
		t.Fatal(err)
	}
}

func TestTls_SyncTime(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("sntp -sS time.apple.com", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:synctime")); err != nil {
		t.Fatal(err)
	}
}

func TestTls_Disks_RendersPerDiskButtons(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// Mix of system noise and a real /Volumes/* mount; only / and
	// /Volumes/Backup should land as buttons.
	h.Fake.On("df -h",
		"Filesystem        Size    Used   Avail Capacity iused ifree %iused  Mounted on\n"+
			"/dev/disk3s1s1   460Gi    17Gi    13Gi    57%    447k  135M    0%   /\n"+
			"devfs            221Ki   221Ki     0Bi   100%     766     0  100%   /dev\n"+
			"/dev/disk3s5     460Gi   409Gi    13Gi    97%    3.2M  135M    2%   /System/Volumes/Data\n"+
			"/dev/disk2s1     500Gi   300Gi   200Gi    60%    5.0k   10k   33%   /Volumes/Backup\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disks")); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	if !strings.Contains(last.Fields["text"], "Tap a disk for actions") {
		t.Errorf("text = %q", last.Fields["text"])
	}
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	// Expect exactly two disk-button rows (one each for / and /Volumes/Backup),
	// each routing to tls:disk:<shortID>.
	diskRows := 0
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "tls:disk:") &&
				!strings.HasPrefix(btn.CallbackData, "tls:disk-") {
				diskRows++
			}
		}
	}
	if diskRows != 2 {
		t.Errorf("expected 2 disk buttons (one for / and one for /Volumes/Backup), got %d", diskRows)
	}
}

func TestTls_DiskDrillDown_ParsesAndRenders(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// Stash the mount in the shortmap so the drill-down can resolve it.
	id := h.Deps.ShortMap.Put("/Volumes/USB")
	h.Fake.On("diskutil info /Volumes/USB",
		"   Volume Name:               BACKUP\n"+
			"   Mount Point:               /Volumes/USB\n"+
			"   Device Node:               /dev/disk5s1\n"+
			"   File System Personality:   ExFAT\n"+
			"   Disk Size:                 64.0 GB (64000000000 Bytes)\n"+
			"   Volume Used Space:         10.0 GB (10000000000 Bytes)\n"+
			"   Container Free Space:      54.0 GB (54000000000 Bytes)\n"+
			"   Removable Media:           Removable\n"+
			"   Device Location:           External\n"+
			"   Solid State:               No\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disk:"+id)); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	text := last.Fields["text"]
	for _, want := range []string{"BACKUP", "64.0 GB", "ExFAT", "/dev/disk5s1", "External", "Removable"} {
		if !strings.Contains(text, want) {
			t.Errorf("drill-down missing %q; got %q", want, text)
		}
	}
	// Removable disk → Eject button must be present.
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	hasEject := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Eject") {
				hasEject = true
			}
		}
	}
	if !hasEject {
		t.Error("removable disk drill-down should expose ⏏ Eject")
	}
}

func TestTls_DiskDrillDown_FixedDiskHidesEject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := h.Deps.ShortMap.Put("/")
	h.Fake.On("diskutil info /",
		"   Volume Name:               Macintosh HD\n"+
			"   Mount Point:               /\n"+
			"   Removable Media:           Fixed\n"+
			"   Device Location:           Internal\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disk:"+id)); err != nil {
		t.Fatal(err)
	}
	kb := telegramtest.MustDecodeInlineKeyboard(t, h.Recorder.Last())
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Eject") {
				t.Error("fixed (non-removable) disk must NOT expose ⏏ Eject")
			}
		}
	}
}

func TestTls_DiskOpen_InvokesOpenCmd(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := h.Deps.ShortMap.Put("/Volumes/USB")
	h.Fake.On("open /Volumes/USB", "", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disk-open:"+id)); err != nil {
		t.Fatal(err)
	}
	// Toast (answerCallbackQuery) is the only side-effect for this action.
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected a toast acknowledging the open")
	}
}

func TestTls_DiskEject_InvokesEjectAndRerendersList(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := h.Deps.ShortMap.Put("/Volumes/USB")
	h.Fake.
		On("diskutil eject /Volumes/USB", "", nil).
		On("df -h",
			"Filesystem      Size  Used Avail Cap iused ifree %iused  Mounted on\n"+
				"/dev/disk1s1   200G  100G  100G 50% 0 0 0% /\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disk-eject:"+id)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "Ejected") {
		t.Errorf("expected 'Ejected …' in re-rendered text; got %q", h.Recorder.Last().Fields["text"])
	}
}

func TestTls_Disk_ExpiredShortMap_FailsCleanly(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disk:nonexistent-id")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "session expired") {
		t.Errorf("expected 'session expired' message; got %q", h.Recorder.Last().Fields["text"])
	}
}

func TestTls_Shortcut_RendersFirstPage(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// 20 shortcuts → 2 pages (15 per page).
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("Shortcut %02d", i))
	}
	h.Fake.On("shortcuts list", strings.Join(lines, "\n")+"\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:shortcut")); err != nil {
		t.Fatal(err)
	}
	last := h.Recorder.Last()
	text := last.Fields["text"]
	for _, want := range []string{"Run Shortcut", "Page 1/2", "20 shortcuts"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
		}
	}
	kb := telegramtest.MustDecodeInlineKeyboard(t, last)
	scRunCount := 0
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "tls:sc-run:") {
				scRunCount++
			}
		}
	}
	if scRunCount != 15 {
		t.Errorf("expected 15 sc-run buttons on page 1, got %d", scRunCount)
	}
}

func TestTls_ShortcutPage_PreservesFilter(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("shortcuts list", "Wifi A\nWifi B\nWifi C\nUnrelated\n", nil)
	// Stash a filter substring in the shortmap; tap sc-page with that id.
	filterID := h.Deps.ShortMap.Put("wifi")
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:sc-page:0:"+filterID)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.Last().Fields["text"]
	for _, want := range []string{"Filtered:", "wifi", "3 match"} {
		if !strings.Contains(text, want) {
			t.Errorf("filtered render missing %q; got %q", want, text)
		}
	}
}

func TestTls_ShortcutRun_InvokesAndRerenders(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.
		On("shortcuts list", "Turn on DND\nOther\n", nil).
		On("shortcuts run Turn on DND", "", nil)
	scID := h.Deps.ShortMap.Put("Turn on DND")
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:sc-run:"+scID+":0:-")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.Last().Fields["text"]
	if !strings.Contains(text, "Ran") || !strings.Contains(text, "Turn on DND") {
		t.Errorf("expected success status containing 'Ran' + name; got %q", text)
	}
}

func TestTls_ShortcutRun_ExpiredShortMap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:sc-run:nope:0:-")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Recorder.Last().Fields["text"], "session expired") {
		t.Errorf("expected session-expired message; got %q", h.Recorder.Last().Fields["text"])
	}
}

func TestTls_ShortcutSearch_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:sc-search")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected search flow installed")
	}
}

func TestTls_ShortcutType_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:sc-type")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected typed-name flow installed")
	}
}

func TestTls_ShortcutGated(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Deps.Capability.Features.Shortcuts = false
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:shortcut"))
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Fatal("expected toast")
	}
}

// ============================ handler utilities ============================

func TestCode(t *testing.T) {
	t.Parallel()
	got := handlers.Code("hello")
	if got != "```\nhello\n```" {
		t.Errorf("got %q", got)
	}
}
