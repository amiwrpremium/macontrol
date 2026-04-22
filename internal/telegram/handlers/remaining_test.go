package handlers_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
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
	text := h.Recorder.Last().Fields["text"]
	for _, want := range []string{"Used:", "24.0 GiB", "(95%)", "Wired:", "Compressed:", "Swap used:", "Pressure:", "Warning", "Top by RAM:", "Chrome", "12.4%"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
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
	text := h.Recorder.Last().Fields["text"]
	for _, want := range []string{"Busy:", "37%", "User", "Kernel", "Idle", "Load avg", "5.41", "12 cores", "Top by CPU:", "12.4%", "Chrome"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q; got %q", want, text)
		}
	}
}

func TestSys_Top(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("ps -Ao pid,pcpu,pmem,comm -r",
		"PID %CPU %MEM COMM\n100 10 5 /App\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "sys:top")); err != nil {
		t.Fatal(err)
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

func TestTls_Disks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On("df -h", "Filesystem Size Used Avail Cap iused ifree %iused Mounted on\n/dev/disk1s1 200G 100G 100G 50% 0 0 0% /\n", nil)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:disks")); err != nil {
		t.Fatal(err)
	}
}

func TestTls_Shortcut_InstallsFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "tls:shortcut")); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Deps.FlowReg.Active(42); !ok {
		t.Fatal("expected flow")
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
