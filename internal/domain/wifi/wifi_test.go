package wifi_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

const hwportsWiFi = `Hardware Port: Ethernet
Device: en3
Ethernet Address: aa:bb:cc:dd:ee:ff

Hardware Port: Wi-Fi
Device: en0
Ethernet Address: aa:bb:cc:dd:ee:00

Hardware Port: Bluetooth PAN
Device: en1
Ethernet Address: aa:bb:cc:dd:ee:01
`

const hwportsAirPort = `Hardware Port: AirPort
Device: en0
`

const hwportsNone = `Hardware Port: Ethernet
Device: en3
`

func TestInterface_FindsWiFi(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -listallhardwareports", hwportsWiFi, nil)
	iface, err := wifi.New(f).Interface(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if iface != "en0" {
		t.Fatalf("iface = %q", iface)
	}
}

func TestInterface_FindsLegacyAirPort(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -listallhardwareports", hwportsAirPort, nil)
	iface, err := wifi.New(f).Interface(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if iface != "en0" {
		t.Fatalf("iface = %q", iface)
	}
}

func TestInterface_NoWiFiPort(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -listallhardwareports", hwportsNone, nil)
	if _, err := wifi.New(f).Interface(context.Background()); err == nil {
		t.Fatal("expected error — no Wi-Fi port")
	}
}

func TestInterface_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -listallhardwareports", "", errors.New("boom"))
	if _, err := wifi.New(f).Interface(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet_PowerOnWithSSID(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.PowerOn || info.SSID != "home" || info.Interface != "en0" {
		t.Fatalf("info = %+v", info)
	}
}

func TestGet_PowerOnNotAssociated(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "You are not associated with an AirPort network.\n", nil)
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.PowerOn {
		t.Error("expected power on")
	}
	if info.SSID != "" {
		t.Errorf("SSID = %q", info.SSID)
	}
}

func TestGet_PowerOffSkipsSSID(t *testing.T) {
	t.Parallel()
	// No rule for getairportnetwork — if the code tried to call it the Fake
	// would error.
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): Off\n", nil)
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.PowerOn {
		t.Error("expected power off")
	}
}

func TestGet_NoInterface(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -listallhardwareports", hwportsNone, nil)
	if _, err := wifi.New(f).Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetPower_On(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportpower en0 on", "", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: x\n", nil)
	info, err := wifi.New(f).SetPower(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.PowerOn {
		t.Error("expected On")
	}
}

func TestSetPower_Off(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportpower en0 off", "", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): Off\n", nil)
	info, err := wifi.New(f).SetPower(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if info.PowerOn {
		t.Error("expected Off")
	}
}

func TestSetPower_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportpower en0 on", "", errors.New("permission denied"))
	if _, err := wifi.New(f).SetPower(context.Background(), true); err == nil {
		t.Fatal("expected error")
	}
}

func TestToggle(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): Off\n", nil).
		On("networksetup -setairportpower en0 on", "", nil)
	_, err := wifi.New(f).Toggle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// At least one 'on' set recorded.
	for _, c := range f.Calls() {
		if c.Name == "networksetup" && len(c.Args) >= 3 && c.Args[0] == "-setairportpower" && c.Args[2] == "on" {
			return
		}
	}
	t.Fatal("expected a setairportpower on call")
}

func TestJoin_WithPassword(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportnetwork en0 home secret", "", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
	info, err := wifi.New(f).Join(context.Background(), "home", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if info.SSID != "home" {
		t.Fatalf("SSID=%q", info.SSID)
	}
}

func TestJoin_NoPassword(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportnetwork en0 open", "", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: open\n", nil)
	_, err := wifi.New(f).Join(context.Background(), "open", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range f.Calls() {
		if c.Name == "networksetup" && len(c.Args) > 0 && c.Args[0] == "-setairportnetwork" {
			if len(c.Args) != 3 {
				t.Fatalf("expected 3 args (no password), got %d: %v", len(c.Args), c.Args)
			}
		}
	}
}

func TestJoin_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportnetwork en0 bad", "", errors.New("Bad password"))
	if _, err := wifi.New(f).Join(context.Background(), "bad", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetDNS_Cloudflare(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -setdnsservers Wi-Fi 1.1.1.1 1.0.0.1", "", nil)
	if err := wifi.New(f).SetDNS(context.Background(), []string{"1.1.1.1", "1.0.0.1"}); err != nil {
		t.Fatal(err)
	}
}

func TestSetDNS_Reset(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -setdnsservers Wi-Fi Empty", "", nil)
	if err := wifi.New(f).SetDNS(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestSetDNS_Empty(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networksetup -setdnsservers Wi-Fi Empty", "", nil)
	if err := wifi.New(f).SetDNS(context.Background(), []string{}); err != nil {
		t.Fatal(err)
	}
}

func TestDiag_InvokesSudoWdutil(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("wdutil info", "SSID: home\nBSSID: xx\n", nil)
	out, err := wifi.New(f).Diag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "SSID") {
		t.Fatalf("output = %q", out)
	}
	// Sanity: Diag used the Sudo path.
	for _, c := range f.Calls() {
		if c.Name == "wdutil" && !c.Sudo {
			t.Fatal("wdutil should be invoked via Sudo")
		}
	}
}

func TestSpeedtest_ParsesMbps(t *testing.T) {
	t.Parallel()
	out := `==== SUMMARY ====
Uplink capacity: 89.721 Mbps (Steady Throughput)
Downlink capacity: 853.432 Mbps (Steady Throughput)
`
	f := runner.NewFake().On("networkQuality -v", out, nil)
	res, err := wifi.New(f).Speedtest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.DownloadMbps < 800 || res.DownloadMbps > 900 {
		t.Errorf("download = %f", res.DownloadMbps)
	}
	if res.UploadMbps < 80 || res.UploadMbps > 100 {
		t.Errorf("upload = %f", res.UploadMbps)
	}
	if !strings.Contains(res.Raw, "SUMMARY") {
		t.Error("raw should contain full output")
	}
}

func TestSpeedtest_MissingFields(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networkQuality -v", "some garbage\n", nil)
	res, err := wifi.New(f).Speedtest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.DownloadMbps != 0 || res.UploadMbps != 0 {
		t.Errorf("expected zero values; got %+v", res)
	}
}

func TestSpeedtest_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("networkQuality -v", "", errors.New("no such cmd"))
	if _, err := wifi.New(f).Speedtest(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
