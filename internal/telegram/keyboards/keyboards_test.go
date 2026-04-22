package keyboards_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
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

// assertNavPresent asserts the last row is the standard Nav (🏠 Home).
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

func TestConfirmRow(t *testing.T) {
	row := keyboards.ConfirmRow(callbacks.NSPower, "shutdown")
	if len(row) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row))
	}
	if !strings.Contains(row[0].Text, "Confirm") {
		t.Errorf("first = %q", row[0].Text)
	}
	if row[0].CallbackData != "pwr:shutdown:ok" {
		t.Errorf("confirm callback = %q", row[0].CallbackData)
	}
	if row[1].CallbackData != "nav:home" {
		t.Errorf("cancel callback = %q", row[1].CallbackData)
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
}

func TestPowerConfirm(t *testing.T) {
	text, kb := keyboards.PowerConfirm("shutdown", "shutdown")
	if !strings.Contains(text, "shutdown") {
		t.Errorf("text = %q", text)
	}
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected single row of 2; got %+v", kb.InlineKeyboard)
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
}

func TestBattery_NotPresent(t *testing.T) {
	text, kb := keyboards.Battery(battery.Status{Present: false, Percent: -1})
	if !strings.Contains(text, "not present") {
		t.Errorf("text = %q", text)
	}
	assertAllRoundtrip(t, kb)
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

// ---------------- Bluetooth ----------------

func TestBluetooth_On(t *testing.T) {
	_, kb := keyboards.Bluetooth(bluetooth.State{PowerOn: true})
	assertContainsButton(t, kb, "Turn off")
	assertContainsButton(t, kb, "Paired")
	assertAllRoundtrip(t, kb)
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
}

// ---------------- Notify ----------------

func TestNotify_Keyboard(t *testing.T) {
	_, kb := keyboards.Notify()
	assertContainsButton(t, kb, "Send notification")
	assertContainsButton(t, kb, "Say")
	assertAllRoundtrip(t, kb)
}

// ---------------- Tools ----------------

func TestTools_WithShortcuts(t *testing.T) {
	_, kb := keyboards.Tools(capability.Features{Shortcuts: true})
	assertContainsButton(t, kb, "Clipboard")
	assertContainsButton(t, kb, "Timezone")
	assertContainsButton(t, kb, "Run Shortcut")
	assertAllRoundtrip(t, kb)
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
