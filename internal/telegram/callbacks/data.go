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
	Namespace string   // e.g. "snd"
	Action    string   // e.g. "up"
	Args      []string // zero or more positional args
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
	NSSound   = "snd"
	NSDisplay = "dsp"
	NSPower   = "pwr"
	NSWifi    = "wif"
	NSBT      = "bt"
	NSBattery = "bat"
	NSSystem  = "sys"
	NSMedia   = "med"
	NSNotify  = "ntf"
	NSTools   = "tls"
	NSNav     = "nav"
)

// AllNamespaces is used by tests to ensure every NS has a registered handler.
var AllNamespaces = []string{
	NSSound, NSDisplay, NSPower, NSWifi, NSBT, NSBattery,
	NSSystem, NSMedia, NSNotify, NSTools, NSNav,
}
