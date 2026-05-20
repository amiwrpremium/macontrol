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

// wdutilFull is a real macOS-26 capture (anonymized) — the sample
// the SSID-from-wdutil parser must handle.
const wdutilFull = `————————————————————————————————————————————————————————————————————
NETWORK
————————————————————————————————————————————————————————————————————
    Primary IPv4         : utun4 ((null) / com.cisco.anyconnect)
                         : 10.201.42.101
    Primary IPv6         : utun4 ((null) / com.cisco.anyconnect)
                         : FE80:0:0:0:506D:B26D:A3E8:6DF9
    DNS Addresses        : 172.21.30.1
                         : 172.20.30.1
    Apple                : Reachable
————————————————————————————————————————————————————————————————————
WIFI
————————————————————————————————————————————————————————————————————
    MAC Address          : 92:c3:5e:ba:38:a4 (hw=92:c3:5e:ba:38:a4)
    Interface Name       : en0
    Power                : On [On]
    Op Mode              : STA
    SSID                 : MyHomeNetwork
    BSSID                : aa:bb:cc:dd:ee:ff
    RSSI                 : -45 dBm
    CCA                  : 33 %
    Noise                : -84 dBm
    Tx Rate              : 144.0 Mbps
    Security             : WPA/WPA2 Personal
    PHY Mode             : 11n
    MCS Index            : 15
    Guard Interval       : 800
    NSS                  : 2
    Channel              : 2g3/20
    Country Code         : IQ
————————————————————————————————————————————————————————————————————
BLUETOOTH
————————————————————————————————————————————————————————————————————
    Power                : On
    Address              : 84:2f:57:20:0f:9d
`

// systemProfilerFull is a real macOS-26 capture (anonymized) — the
// sample the SSID-from-system_profiler parser must handle.
const systemProfilerFull = `Wi-Fi:

      Software Versions:
          CoreWLAN: 16.0 (1657)
      Interfaces:
        en0:
          Card Type: Wi-Fi  (0x14E4, 0x4388)
          MAC Address: 92:c3:5e:ba:38:a4
          Status: Connected
          Current Network Information:
            MyHomeNetwork:
              PHY Mode: 802.11n
              Channel: 3 (2GHz, 20MHz)
              Country Code: IQ
              Network Type: Infrastructure
              Security: WPA/WPA2 Personal
              Signal / Noise: -48 dBm / -84 dBm
              Transmit Rate: 144
              MCS Index: 15
          Other Local Wi-Fi Networks:
            OtherNetA:
              PHY Mode: 802.11b/g/n
              Channel: 4 (2GHz, 20MHz)
              Network Type: Infrastructure
              Security: WPA/WPA2 Personal
            OtherNetB:
              PHY Mode: 802.11b/g/n
              Channel: 7 (2GHz, 20MHz)
              Network Type: Infrastructure
              Security: WPA/WPA2 Personal
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

func TestGet_PowerOnSSIDFromWdutil(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("wdutil info", wdutilFull, nil)
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.PowerOn {
		t.Error("expected PowerOn")
	}
	if info.SSID != "MyHomeNetwork" {
		t.Errorf("SSID = %q", info.SSID)
	}
	if info.BSSID != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("BSSID = %q", info.BSSID)
	}
	if info.RSSI != -45 {
		t.Errorf("RSSI = %d", info.RSSI)
	}
	if info.Security != "WPA/WPA2 Personal" {
		t.Errorf("Security = %q", info.Security)
	}
	if info.TxRateMbps != 144.0 {
		t.Errorf("TxRateMbps = %f", info.TxRateMbps)
	}
	if info.Channel != "2g3/20" {
		t.Errorf("Channel = %q", info.Channel)
	}
}

func TestGet_PowerOnFallsBackToSystemProfiler(t *testing.T) {
	t.Parallel()
	// wdutil errors (sudoers not installed); system_profiler succeeds.
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("wdutil info", "", errors.New("sudo: a password is required")).
		On("system_profiler SPAirPortDataType", systemProfilerFull, nil)
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.SSID != "MyHomeNetwork" {
		t.Errorf("SSID = %q", info.SSID)
	}
	if info.Channel != "3 (2GHz, 20MHz)" {
		t.Errorf("Channel = %q", info.Channel)
	}
	if info.Security != "WPA/WPA2 Personal" {
		t.Errorf("Security = %q", info.Security)
	}
	if info.RSSI != -48 {
		t.Errorf("RSSI = %d", info.RSSI)
	}
	if info.TxRateMbps != 144 {
		t.Errorf("TxRateMbps = %f", info.TxRateMbps)
	}
}

func TestGet_PowerOnBothSourcesFail(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("wdutil info", "", errors.New("nope")).
		On("system_profiler SPAirPortDataType", "", errors.New("nope"))
	info, err := wifi.New(f).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.PowerOn {
		t.Error("expected PowerOn")
	}
	if info.SSID != "" {
		t.Errorf("SSID = %q (want empty)", info.SSID)
	}
}

func TestGet_PowerOffSkipsSSID(t *testing.T) {
	t.Parallel()
	// No rule for wdutil/system_profiler — if Get tried to call either the
	// Fake would error; passing means we short-circuited on PowerOn=false.
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
		On("wdutil info", wdutilFull, nil)
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
		On("wdutil info", wdutilFull, nil)
	info, err := wifi.New(f).Join(context.Background(), "home", "secret")
	if err != nil {
		t.Fatal(err)
	}
	// SSID populated from wdutil sample (MyHomeNetwork), not the joined SSID.
	// We're only verifying Join propagates and Get is invoked successfully.
	if info.SSID == "" {
		t.Error("SSID should be populated")
	}
}

func TestJoin_NoPassword(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("networksetup -listallhardwareports", hwportsWiFi, nil).
		On("networksetup -setairportnetwork en0 open", "", nil).
		On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil).
		On("wdutil info", wdutilFull, nil)
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
