// Package callbacks owns the inline-keyboard callback protocol: encoding
// short `<ns>:<action>[:<arg>]` strings, dispatching to per-namespace
// handlers, and the shortmap that stores overflow args keyed by short id.
package callbacks

import (
	"fmt"
	"strings"
)

// MaxCallbackDataBytes is Telegram's hard cap for InlineKeyboardButton.CallbackData.
const MaxCallbackDataBytes = 64

// Data is the parsed form of a callback_data string.
type Data struct {
	// Namespace is the namespace prefix, e.g. "snd" for the sound
	// handler.
	Namespace string
	// Action is the verb inside the namespace, e.g. "up".
	Action string
	// Args carries zero or more colon-separated positional args
	// pulled from the remainder of the callback data.
	Args []string
}

// Encode renders the Data back to a string, panicking if the result would
// exceed MaxCallbackDataBytes — callers must guarantee short inputs. Use a
// shortmap for arguments that might overflow.
func Encode(ns, action string, args ...string) string {
	parts := make([]string, 0, 2+len(args))
	parts = append(parts, ns, action)
	parts = append(parts, args...)
	s := strings.Join(parts, ":")
	if len(s) > MaxCallbackDataBytes {
		panic(fmt.Sprintf("callback data exceeds %d bytes: %q", MaxCallbackDataBytes, s))
	}
	return s
}

// Decode parses a callback_data string. Returns an error if the string is
// empty, malformed, or longer than MaxCallbackDataBytes.
func Decode(raw string) (Data, error) {
	if raw == "" {
		return Data{}, fmt.Errorf("empty callback data")
	}
	if len(raw) > MaxCallbackDataBytes {
		return Data{}, fmt.Errorf("callback data exceeds %d bytes", MaxCallbackDataBytes)
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return Data{}, fmt.Errorf("callback data must have at least namespace and action: %q", raw)
	}
	d := Data{Namespace: parts[0], Action: parts[1]}
	if len(parts) > 2 {
		d.Args = parts[2:]
	}
	return d, nil
}

// Namespace constants are used both for encoding and routing.
const (
	// NSSound is the namespace for volume/mute/say callbacks.
	NSSound = "snd"
	// NSDisplay is the namespace for brightness callbacks.
	NSDisplay = "dsp"
	// NSPower is the namespace for sleep/lock/restart/shutdown/
	// caffeinate callbacks.
	NSPower = "pwr"
	// NSWifi is the namespace for Wi-Fi state, join, and DNS
	// callbacks.
	NSWifi = "wif"
	// NSBT is the namespace for Bluetooth callbacks.
	NSBT = "bt"
	// NSBattery is the namespace for battery status callbacks.
	NSBattery = "bat"
	// NSSystem is the namespace for system info, memory, CPU, and
	// process callbacks.
	NSSystem = "sys"
	// NSMedia is the namespace for screenshot/record/photo callbacks.
	NSMedia = "med"
	// NSNotify is the namespace for desktop-notification and say
	// callbacks.
	NSNotify = "ntf"
	// NSTools is the namespace for clipboard, timezone, disks, and
	// Shortcuts callbacks.
	NSTools = "tls"
	// NSNav is the namespace for cross-cutting navigation (back
	// to home, refresh).
	NSNav = "nav"
)

// AllNamespaces is used by tests to ensure every NS has a registered handler.
var AllNamespaces = []string{
	NSSound, NSDisplay, NSPower, NSWifi, NSBT, NSBattery,
	NSSystem, NSMedia, NSNotify, NSTools, NSNav,
}
