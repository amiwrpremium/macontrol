package keyboards_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// allCallbackData returns every button's callback_data string in a keyboard.
func allCallbackData(kb *models.InlineKeyboardMarkup) []string {
	if kb == nil {
		return nil
	}
	out := []string{}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != "" {
				out = append(out, btn.CallbackData)
			}
		}
	}
	return out
}

// assertAllRoundtrip checks every callback_data parses through callbacks.Decode
// and is ≤ 64 bytes.
func assertAllRoundtrip(t *testing.T, kb *models.InlineKeyboardMarkup) {
	t.Helper()
	for _, raw := range allCallbackData(kb) {
		if _, err := callbacks.Decode(raw); err != nil {
			t.Errorf("callback_data %q does not decode: %v", raw, err)
		}
		if len(raw) > callbacks.MaxCallbackDataBytes {
			t.Errorf("callback_data %q exceeds 64B (%d)", raw, len(raw))
		}
	}
}

// assertContainsButton asserts that at least one button has text matching
// the substring.
func assertContainsButton(t *testing.T, kb *models.InlineKeyboardMarkup, substr string) {
	t.Helper()
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, substr) {
				return
			}
		}
	}
	t.Errorf("no button containing %q in keyboard", substr)
}

// assertNavPresent asserts the last row contains the always-on Home
// button.
func assertNavPresent(t *testing.T, kb *models.InlineKeyboardMarkup) {
	t.Helper()
	if len(kb.InlineKeyboard) == 0 {
		t.Fatal("empty keyboard")
	}
	last := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	foundHome := false
	for _, b := range last {
		if strings.Contains(b.Text, "Home") && b.CallbackData == "nav:home" {
			foundHome = true
		}
	}
	if !foundHome {
		t.Errorf("final row missing Home button: %+v", last)
	}
}

// assertBackPresent asserts the keyboard contains a "← Back" button
// somewhere. Every nested menu (anything reached from the home grid
// or deeper) must surface this affordance.
func assertBackPresent(t *testing.T, kb *models.InlineKeyboardMarkup) {
	t.Helper()
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.Contains(b.Text, "Back") {
				return
			}
		}
	}
	t.Errorf("keyboard missing Back button (every nested menu should have one)")
}

// ---------------- Home ----------------

func TestInlineHome_Roundtrips(t *testing.T) {
	assertAllRoundtrip(t, keyboards.InlineHome())
}

func TestInlineHome_EveryCategoryHasOpenButton(t *testing.T) {
	kb := keyboards.InlineHome()
	found := map[string]bool{}
	for _, raw := range allCallbackData(kb) {
		d, _ := callbacks.Decode(raw)
		if d.Action == "open" {
			found[d.Namespace] = true
		}
	}
	for _, c := range keyboards.Categories {
		if !found[c.Namespace] {
			t.Errorf("no open button for namespace %s", c.Namespace)
		}
	}
}

func TestCategories_NamespacesKnown(t *testing.T) {
	known := map[string]bool{}
	for _, ns := range callbacks.AllNamespaces {
		known[ns] = true
	}
	for _, c := range keyboards.Categories {
		if !known[c.Namespace] {
			t.Errorf("category %q uses unknown namespace %q", c.Label, c.Namespace)
		}
	}
}

// ---------------- Common ----------------

func TestNav_ContainsHome(t *testing.T) {
	row := keyboards.Nav()
	if len(row) == 0 {
		t.Fatal("empty nav row")
	}
	if !strings.Contains(row[0].Text, "Home") {
		t.Errorf("first nav button = %q", row[0].Text)
	}
	if row[0].CallbackData != "nav:home" {
		t.Errorf("callback = %q", row[0].CallbackData)
	}
}

func TestNavWithBack_BackBeforeHome(t *testing.T) {
	row := keyboards.NavWithBack(callbacks.NSPower, "open")
	if len(row) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row))
	}
	if !strings.Contains(row[0].Text, "Back") {
		t.Errorf("first = %q (want Back)", row[0].Text)
	}
	if row[0].CallbackData != "pwr:open" {
		t.Errorf("back callback = %q", row[0].CallbackData)
	}
	if !strings.Contains(row[1].Text, "Home") {
		t.Errorf("second = %q (want Home)", row[1].Text)
	}
	if row[1].CallbackData != "nav:home" {
		t.Errorf("home callback = %q", row[1].CallbackData)
	}
}

func TestConfirmRow_CancelGoesToParent(t *testing.T) {
	// PowerConfirm-style: confirm fires the destructive action with "ok",
	// cancel returns to the parent dashboard (NOT all the way Home).
	row := keyboards.ConfirmRow(callbacks.NSPower, "shutdown", callbacks.NSPower, "open")
	if len(row) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row))
	}
	if row[0].CallbackData != "pwr:shutdown:ok" {
		t.Errorf("confirm callback = %q", row[0].CallbackData)
	}
	if row[1].CallbackData != "pwr:open" {
		t.Errorf("cancel callback = %q (regression: must NOT route to nav:home)",
			row[1].CallbackData)
	}
}

func TestSingleRow(t *testing.T) {
	kb := keyboards.SingleRow(
		models.InlineKeyboardButton{Text: "A", CallbackData: "x:y"},
		models.InlineKeyboardButton{Text: "B", CallbackData: "x:z"},
	)
	if len(kb) != 1 || len(kb[0]) != 2 {
		t.Fatalf("unexpected shape: %+v", kb)
	}
}

// ---------------- Sound ----------------

func TestSound_Unmuted(t *testing.T) {
	text, kb := keyboards.Sound(sound.State{Level: 60, Muted: false})
	if !strings.Contains(text, "60%") || !strings.Contains(text, "unmuted") {
		t.Errorf("text = %q", text)
	}
	assertContainsButton(t, kb, "🔇 Mute")
	assertAllRoundtrip(t, kb)
	assertNavPresent(t, kb)
	assertBackPresent(t, kb)
}

func TestSound_Muted(t *testing.T) {
	text, kb := keyboards.Sound(sound.State{Level: 0, Muted: true})
	if !strings.Contains(text, "MUTED") {
		t.Errorf("text = %q", text)
	}
	assertContainsButton(t, kb, "🔈 Unmute")
	assertAllRoundtrip(t, kb)
}

// ---------------- Display ----------------

func TestDisplay_WithLevel(t *testing.T) {
	text, kb := keyboards.Display(display.State{Level: 0.5}, nil)
	if !strings.Contains(text, "50%") {
		t.Errorf("text = %q", text)
	}
	assertAllRoundtrip(t, kb)
	assertNavPresent(t, kb)
	assertBackPresent(t, kb)
}

func TestDisplay_ErrorSurfaced(t *testing.T) {
	text, kb := keyboards.Display(display.State{Level: -1},
		errors.New("brightness: failed (error -536870201)"))
	if !strings.Contains(text, "level unknown") {
		t.Errorf("text = %q", text)
	}
	if !strings.Contains(text, "-536870201") {
		t.Errorf("expected error text in dashboard, got %q", text)
	}
	// Buttons must still be present so the user can try ± anyway.
	assertContainsButton(t, kb, "+5")
}

func TestDisplay_UnknownNoError(t *testing.T) {
	text, _ := keyboards.Display(display.State{Level: -1}, nil)
	if !strings.Contains(text, "level unknown") {
		t.Errorf("text = %q", text)
	}
}

// ---------------- Power ----------------

func TestPower_HasAllActions(t *testing.T) {
	_, kb := keyboards.Power()
	for _, word := range []string{"Lock", "Sleep", "Restart", "Shutdown", "Logout", "Keep awake"} {
		assertContainsButton(t, kb, word)
	}
	assertAllRoundtrip(t, kb)
	assertNavPresent(t, kb)
	assertBackPresent(t, kb)
}

func TestPowerConfirm(t *testing.T) {
	text, kb := keyboards.PowerConfirm("shutdown", "shutdown")
	if !strings.Contains(text, "shutdown") {
		t.Errorf("text = %q", text)
	}
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected single row of 2; got %+v", kb.InlineKeyboard)
	}
	// Cancel must return to the Power dashboard, not all the way Home.
	cancel := kb.InlineKeyboard[0][1]
	if cancel.CallbackData != "pwr:open" {
		t.Errorf("Cancel callback = %q; want pwr:open (was nav:home before fix)",
			cancel.CallbackData)
	}
	assertAllRoundtrip(t, kb)
}

// ---------------- Battery ----------------

func TestBattery_Present(t *testing.T) {
	text, kb := keyboards.Battery(battery.Status{
		Percent: 80, State: battery.StateCharging, TimeRemaining: "1:00",
		Present: true,
	})
	if !strings.Contains(text, "80%") {
		t.Errorf("text = %q", text)
	}
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Health")
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

func TestBattery_NotPresent(t *testing.T) {
	text, kb := keyboards.Battery(battery.Status{Present: false, Percent: -1})
	if !strings.Contains(text, "not present") {
		t.Errorf("text = %q", text)
	}
	assertAllRoundtrip(t, kb)
}

func TestBatteryHealthPanel_RefreshAndBack(t *testing.T) {
	kb := keyboards.BatteryHealthPanel()
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertNavPresent(t, kb)
	assertAllRoundtrip(t, kb)
	// Health drill-down must NOT carry the original Health button.
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Health") {
				t.Error("Health button should be absent from the health drill-down panel")
			}
		}
	}
}

func TestBattery_IconByState(t *testing.T) {
	cases := []struct {
		pct   int
		state battery.ChargeState
	}{
		{95, battery.StateCharging},
		{85, battery.StateDischarging},
		{50, battery.StateDischarging},
		{25, battery.StateDischarging},
		{5, battery.StateDischarging},
	}
	for _, c := range cases {
		text, _ := keyboards.Battery(battery.Status{
			Percent: c.pct, State: c.state, Present: true,
		})
		if text == "" {
			t.Errorf("empty for %+v", c)
		}
	}
}

// ---------------- Wi-Fi ----------------

func TestWiFi_OnAssociated(t *testing.T) {
	feat := capability.Features{NetworkQuality: true}
	text, kb := keyboards.WiFi(wifi.Info{Interface: "en0", PowerOn: true, SSID: "home"}, feat)
	if !strings.Contains(text, "home") {
		t.Errorf("text = %q", text)
	}
	assertContainsButton(t, kb, "Turn off")
	assertContainsButton(t, kb, "Speed test")
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

func TestWiFi_Off(t *testing.T) {
	feat := capability.Features{NetworkQuality: false}
	text, kb := keyboards.WiFi(wifi.Info{Interface: "en0", PowerOn: false}, feat)
	if !strings.Contains(text, "off") {
		t.Errorf("text = %q", text)
	}
	assertContainsButton(t, kb, "Turn on")
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Speed test") {
				t.Error("Speed test should be hidden when feature disabled")
			}
		}
	}
}

func TestWiFi_OnNotAssociated(t *testing.T) {
	text, _ := keyboards.WiFi(wifi.Info{Interface: "en0", PowerOn: true, SSID: ""},
		capability.Features{})
	if !strings.Contains(text, "not associated") {
		t.Errorf("text = %q", text)
	}
}

func TestWiFiDiagPanel_RefreshAndBack(t *testing.T) {
	kb := keyboards.WiFiDiagPanel()
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertNavPresent(t, kb)
	assertAllRoundtrip(t, kb)
	// Diagnostics drill-down must NOT carry the original Info button.
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Info") {
				t.Error("Info button should be absent from the diagnostics drill-down panel")
			}
		}
	}
}

// ---------------- Bluetooth ----------------

func TestBluetooth_On(t *testing.T) {
	_, kb := keyboards.Bluetooth(bluetooth.State{PowerOn: true})
	assertContainsButton(t, kb, "Turn off")
	assertContainsButton(t, kb, "Paired")
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

func TestBluetooth_Off(t *testing.T) {
	_, kb := keyboards.Bluetooth(bluetooth.State{PowerOn: false})
	assertContainsButton(t, kb, "Turn on")
}

func TestBluetoothDevices_Empty(t *testing.T) {
	text, kb := keyboards.BluetoothDevices(nil)
	if !strings.Contains(text, "No paired devices") {
		t.Errorf("text = %q", text)
	}
	assertAllRoundtrip(t, kb)
	// Empty state must still let the user back out — Back to bt:open.
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

func TestBluetoothDevices_List(t *testing.T) {
	rows := []keyboards.BluetoothDeviceRow{
		{Label: "AirPods", ShortID: "abc", Connected: true},
		{Label: "Keyboard", ShortID: "def", Connected: false},
	}
	_, kb := keyboards.BluetoothDevices(rows)
	assertContainsButton(t, kb, "AirPods")
	assertContainsButton(t, kb, "Keyboard")
	// Scissors for connected, link for disconnected.
	assertContainsButton(t, kb, "✂")
	assertContainsButton(t, kb, "🔗")
	assertAllRoundtrip(t, kb)
}

// ---------------- System ----------------

func TestSystem_Menu(t *testing.T) {
	_, kb := keyboards.System()
	for _, word := range []string{"Info", "Temperature", "Memory", "CPU", "Top", "Kill"} {
		assertContainsButton(t, kb, word)
	}
	assertAllRoundtrip(t, kb)
	assertNavPresent(t, kb)
	assertBackPresent(t, kb)
}

func TestSystemPanelWithProcs_PerProcessButtons(t *testing.T) {
	procs := []system.Process{
		{PID: 100, CPU: 50.0, Mem: 10.0, Command: "/Apps/A"},
		{PID: 200, CPU: 25.0, Mem: 5.0, Command: "/Apps/B"},
	}
	label := func(p system.Process) string {
		return strconv.Itoa(p.PID) + " · X"
	}
	kb := keyboards.SystemPanelWithProcs("cpu", procs, label)
	// First two rows are per-process buttons routing to sys:proc:<pid>.
	if len(kb.InlineKeyboard) < 4 {
		t.Fatalf("expected ≥4 rows (2 procs + Refresh/Back + Home), got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].CallbackData != "sys:proc:100" {
		t.Errorf("first proc callback = %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	if kb.InlineKeyboard[1][0].CallbackData != "sys:proc:200" {
		t.Errorf("second proc callback = %q", kb.InlineKeyboard[1][0].CallbackData)
	}
	// Refresh on the panel's own action.
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertNavPresent(t, kb)
	assertAllRoundtrip(t, kb)
}

func TestSystemPanelWithProcs_EmptyDegradesToPlainPanel(t *testing.T) {
	kb := keyboards.SystemPanelWithProcs("mem", nil, func(p system.Process) string { return "x" })
	// No process rows → just Refresh/Back + Home.
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows when procs nil, got %d", len(kb.InlineKeyboard))
	}
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertNavPresent(t, kb)
}

func TestSystemPanel(t *testing.T) {
	kb := keyboards.SystemPanel("temp")
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertAllRoundtrip(t, kb)
}

// ---------------- Media ----------------

func TestMedia_AllButtons(t *testing.T) {
	_, kb := keyboards.Media()
	for _, word := range []string{"Screenshot", "Silent", "Record", "Webcam"} {
		assertContainsButton(t, kb, word)
	}
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

// ---------------- Notify ----------------

func TestNotify_Keyboard(t *testing.T) {
	_, kb := keyboards.Notify()
	assertContainsButton(t, kb, "Send notification")
	assertContainsButton(t, kb, "Say")
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

// ---------------- Tools ----------------

func TestToolsDisksList_PerDiskButtons(t *testing.T) {
	rows := []keyboards.ToolsDiskRow{
		{Mount: "/", Size: "460Gi", Capacity: "54%", ShortID: "abc"},
		{Mount: "/Volumes/Backup", Size: "2Ti", Capacity: "38%", ShortID: "def"},
	}
	kb := keyboards.ToolsDisksList(rows)
	// First two rows are per-disk buttons routing to tls:disk:<shortID>.
	if len(kb.InlineKeyboard) < 4 {
		t.Fatalf("expected ≥4 rows (2 disks + Refresh/Back + Home), got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].CallbackData != "tls:disk:abc" {
		t.Errorf("first disk callback = %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	if !strings.Contains(kb.InlineKeyboard[0][0].Text, "/") || !strings.Contains(kb.InlineKeyboard[0][0].Text, "460Gi") {
		t.Errorf("first disk label missing mount or size: %q", kb.InlineKeyboard[0][0].Text)
	}
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back")
	assertNavPresent(t, kb)
	assertAllRoundtrip(t, kb)
}

func TestToolsDiskPanel_RemovableShowsEject(t *testing.T) {
	kb := keyboards.ToolsDiskPanel("xyz", true)
	assertContainsButton(t, kb, "Open in Finder")
	assertContainsButton(t, kb, "Eject")
	assertContainsButton(t, kb, "Refresh")
	assertContainsButton(t, kb, "Back to Disks")
	assertNavPresent(t, kb)
	assertAllRoundtrip(t, kb)
}

func TestToolsDiskPanel_FixedHidesEject(t *testing.T) {
	kb := keyboards.ToolsDiskPanel("xyz", false)
	assertContainsButton(t, kb, "Open in Finder")
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Eject") {
				t.Error("fixed disk must NOT show Eject button")
			}
		}
	}
}

func TestTools_WithShortcuts(t *testing.T) {
	_, kb := keyboards.Tools(capability.Features{Shortcuts: true})
	assertContainsButton(t, kb, "Clipboard")
	assertContainsButton(t, kb, "Timezone")
	assertContainsButton(t, kb, "Run Shortcut")
	assertAllRoundtrip(t, kb)
	assertBackPresent(t, kb)
	assertNavPresent(t, kb)
}

func TestTools_WithoutShortcuts(t *testing.T) {
	_, kb := keyboards.Tools(capability.Features{Shortcuts: false})
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Run Shortcut") {
				t.Error("Shortcut button should be hidden without feature")
			}
		}
	}
}
