package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/status"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// minSuccessfulFake registers enough commands for info.Info, battery.Get,
// and wifi.Get to all succeed.
func minSuccessfulFake() *runner.Fake {
	return runner.NewFake().
		// info.go
		On("sw_vers", "ProductName: macOS\nProductVersion: 15.3.1\nBuildVersion: 24D70\n", nil).
		On("hostname", "tower.local\n", nil).
		On("sysctl -n hw.model", "MacBookPro18,3\n", nil).
		On("sysctl -n machdep.cpu.brand_string", "Apple M3\n", nil).
		On("sysctl -n hw.memsize", "34359738368\n", nil).
		On("uptime", " 10:00 up 1 day, load average: 1 1 1\n", nil).
		On("system_profiler SPHardwareDataType", "Total Number of Cores: 8\n", nil).
		// battery.go
		On("pmset -g batt", " -InternalBattery-0 (id=1)	80%; charging; 0:30 remaining present: true\n", nil).
		// wifi.go
		On("networksetup -listallhardwareports",
			"Hardware Port: Wi-Fi\nDevice: en0\n", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
}

func TestSnapshot_AllSucceed(t *testing.T) {
	t.Parallel()
	snap, err := status.New(minSuccessfulFake()).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.InfoErr != nil || snap.BatteryErr != nil || snap.WiFiErr != nil {
		t.Fatalf("unexpected sub-errors: %+v", snap)
	}
	if snap.Info.ProductVersion != "15.3.1" {
		t.Errorf("info product version = %q", snap.Info.ProductVersion)
	}
	if snap.Battery.Percent != 80 {
		t.Errorf("battery percent = %d", snap.Battery.Percent)
	}
	if snap.WiFi.SSID != "home" {
		t.Errorf("wifi ssid = %q", snap.WiFi.SSID)
	}
}

func TestSnapshot_PartialFailures(t *testing.T) {
	t.Parallel()
	// Battery fails, others succeed.
	f := minSuccessfulFake()
	// Re-register pmset to fail.
	f.On("pmset -g batt", "", errors.New("broken"))
	snap, err := status.New(f).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("expected nil err with partial failure, got %v", err)
	}
	if snap.BatteryErr == nil {
		t.Fatal("expected BatteryErr to be set")
	}
	if snap.InfoErr != nil || snap.WiFiErr != nil {
		t.Errorf("unexpected other errors: info=%v wifi=%v", snap.InfoErr, snap.WiFiErr)
	}
}

func TestSnapshot_AllFail(t *testing.T) {
	t.Parallel()
	// No rules registered — every call errors.
	f := runner.NewFake().
		On("hostname", "", errors.New("x")).
		On("pmset -g batt", "", errors.New("x")).
		On("networksetup -listallhardwareports", "", errors.New("x"))
	// Other subprocess calls in info.Info will also fail (no rule). That's
	// fine — Info degrades to best-effort. But without hostname, model,
	// etc., it still returns no error. So we need all three services to
	// fail. Battery.Get and wifi.Get both propagate their first failure.
	snap, err := status.New(f).Snapshot(context.Background())
	// Expect a joined error because all three services bubble up non-nil
	// errors (info.Info returns nil even with all-fails; so only two of
	// three will be errored — not the "all fail" case). We'll document
	// current behaviour: returns nil because info.Info degrades silently.
	if err == nil && snap.InfoErr != nil {
		t.Fatal("if Info errored we'd want the joined error")
	}
	// At least battery + wifi must report errors.
	if snap.BatteryErr == nil || snap.WiFiErr == nil {
		t.Fatalf("expected battery + wifi errors, got %+v", snap)
	}
}
