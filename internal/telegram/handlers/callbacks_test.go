package handlers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

func TestCallbackRouter_InvalidData(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id1", ""))
	if err == nil {
		t.Fatal("expected decode error")
	}
	// The router answers the callback with a toast.
	if len(h.Recorder.ByMethod("answerCallbackQuery")) == 0 {
		t.Error("expected answerCallbackQuery")
	}
}

func TestCallbackRouter_UnknownNamespace(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id1", "xxx:open"))
	if err == nil {
		t.Fatal("expected error for unknown namespace")
	}
}

func TestCallbackRouter_NonCallbackUpdate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		&models.Update{})
	if err == nil {
		t.Fatal("expected error for non-callback update")
	}
}

func TestCallbackRouter_DispatchesEveryNamespace(t *testing.T) {
	t.Parallel()
	namespaces := []string{"snd", "dsp", "pwr", "bat", "wif", "bt", "sys", "med", "ntf", "tls", "nav"}
	for _, ns := range namespaces {
		ns := ns
		t.Run(ns, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			// Register enough Fake rules for "open" to succeed where possible.
			seedOpenRules(h)

			data := ns + ":open"
			if ns == "nav" {
				data = "nav:home"
			}
			// We don't care about the specific result — just that no
			// unexpected panic or "unknown namespace" error occurs.
			err := handlers.NewCallbackRouter().Handle(context.Background(),
				h.Deps, newCallbackUpdate("id", data))
			// Most actions reach the service and may error depending on
			// registered rules. We accept any outcome that doesn't claim
			// the namespace is unknown.
			if err != nil && strings.Contains(err.Error(), "unknown callback namespace") {
				t.Fatalf("namespace %s was not routed", ns)
			}
		})
	}
}

// seedOpenRules registers runner rules that let every category's "open"
// action succeed. Used only for the dispatch-smoke-test above.
func seedOpenRules(h *harness) {
	h.Fake.On(soundGetScript, "50,false", nil)
	h.Fake.On("brightness -l", "display 0: brightness 0.500\n", nil)
	h.Fake.On("pmset -g batt", " -InternalBattery-0 (id=1)	80%; charging; 0:30 remaining\n", nil)
	h.Fake.On("networksetup -listallhardwareports",
		"Hardware Port: Wi-Fi\nDevice: en0\n", nil)
	h.Fake.On("networksetup -getairportpower en0", "Wi-Fi Power (en0): On\n", nil)
	h.Fake.On("networksetup -getairportnetwork en0", "Current Wi-Fi Network: home\n", nil)
	h.Fake.On("blueutil -p", "1\n", nil)
}

const soundGetScript = "osascript -e set v to output volume of (get volume settings)\n" +
	"set m to output muted of (get volume settings)\n" +
	"return (v as text) & \",\" & (m as text)"
