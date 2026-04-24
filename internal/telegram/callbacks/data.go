// Package callbacks owns the inline-keyboard callback protocol that
// glues every Telegram tap to the matching domain action.
//
// The protocol is a colon-delimited string the keyboard layer
// stamps into [models.InlineKeyboardButton.CallbackData] and the
// dispatcher parses on every incoming tap:
//
//	<namespace>:<action>[:<arg>[:<arg>…]]
//
// Telegram caps callback_data at 64 bytes ([MaxCallbackDataBytes]),
// so keyboards that need to round-trip long values (full file
// paths, IANA timezone names, Shortcut titles) park the value in a
// [ShortMap] and embed only the short id.
//
// The package exposes:
//
//   - [Encode] / [Decode] — the wire format helpers.
//   - [Data] — the parsed form handlers receive.
//   - The NS… constants and [AllNamespaces] — the namespace
//     vocabulary every keyboard and handler agrees on.
//   - [ShortMap] (shortmap.go) — the side table for overflow values.
package callbacks

import (
	"fmt"
	"strings"
)

// MaxCallbackDataBytes is Telegram's hard cap for
// [models.InlineKeyboardButton.CallbackData]. Both [Encode] and
// [Decode] enforce it: encoding panics on overflow because it
// signals a bug in the keyboard layer, while decoding rejects
// overflow as a malformed input.
const MaxCallbackDataBytes = 64

// Data is the parsed form of a callback_data string. Handlers
// receive this after the dispatcher has matched the namespace and
// stripped it from the raw string.
//
// Lifecycle:
//   - Constructed by [Decode] on every incoming
//     update.callback_query and routed to the per-namespace
//     handler keyed by [Data.Namespace].
//   - Read-only from the handler's perspective; never mutated
//     after parse.
//
// Field roles:
//   - Namespace selects which handler runs (one per dashboard
//     category, plus "nav" for cross-cutting navigation).
//   - Action selects the verb inside that handler — usually one
//     of "open", "refresh", or a feature-specific token like
//     "up", "down", "set", "dns-menu".
//   - Args carries any positional payload the keyboard stamped
//     into the callback. Examples: a brightness step ("5"), a
//     ShortMap id resolving to a long timezone name, a paginator
//     page index.
type Data struct {
	// Namespace is the namespace prefix that selects the handler,
	// e.g. "snd" for the sound dashboard. Always one of the NS…
	// constants; the dispatcher rejects unknown values.
	Namespace string

	// Action is the verb inside the namespace, e.g. "up", "set",
	// "dns-menu". Free-form per handler — no enum.
	Action string

	// Args carries zero or more colon-separated positional args
	// pulled from the remainder of the callback data. Empty when
	// the encoded string was just "ns:action".
	Args []string
}

// Encode renders ns, action, and args back to a callback_data
// string by joining them with single colons. The keyboard layer
// uses this when stamping every [models.InlineKeyboardButton].
//
// Behavior:
//   - Joins parts as "ns:action[:arg[:arg…]]".
//   - Panics if the rendered string exceeds [MaxCallbackDataBytes];
//     that condition signals a bug in the keyboard caller (the
//     args slice should have been moved through a [ShortMap]
//     instead).
//
// Returns the rendered string. Never returns an error — overflow
// is treated as programmer error, not runtime input.
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

// Decode parses a callback_data string into [Data]. Called by the
// dispatcher on every incoming callback_query.
//
// Behavior:
//   - Splits raw on ':' and assigns parts[0] → Namespace,
//     parts[1] → Action, parts[2:] → Args.
//   - Returns an error (and a zero [Data]) when raw is empty,
//     longer than [MaxCallbackDataBytes], or contains fewer than
//     two colon-separated parts.
//
// Returns the parsed [Data] on success, otherwise zero [Data] and
// a non-nil error describing the malformed input.
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

// Namespace constants are the vocabulary every keyboard stamps
// and every dispatcher consults. Adding a new dashboard category
// means adding a constant here, listing it in [AllNamespaces],
// and registering a handler. Values are short on purpose — they
// burn against the 64-byte callback_data budget.
const (
	// NSSound is the namespace for volume / mute / say callbacks.
	// Handler: internal/telegram/handlers/snd.go.
	NSSound = "snd"

	// NSDisplay is the namespace for brightness and screensaver
	// callbacks. Handler: internal/telegram/handlers/dsp.go.
	NSDisplay = "dsp"

	// NSPower is the namespace for sleep / lock / restart /
	// shutdown / caffeinate callbacks. Handler:
	// internal/telegram/handlers/pwr.go.
	NSPower = "pwr"

	// NSWifi is the namespace for Wi-Fi state, join, info, DNS
	// presets, and speed test callbacks. Handler:
	// internal/telegram/handlers/wif.go.
	NSWifi = "wif"

	// NSBT is the namespace for Bluetooth toggle and paired-device
	// callbacks. Handler: internal/telegram/handlers/bt.go.
	NSBT = "bt"

	// NSBattery is the namespace for battery state and health
	// callbacks. Handler: internal/telegram/handlers/bat.go.
	NSBattery = "bat"

	// NSSystem is the namespace for system info, memory, CPU,
	// process-list, and process-kill callbacks. Handler:
	// internal/telegram/handlers/sys.go.
	NSSystem = "sys"

	// NSMedia is the namespace for screenshot, screen-record, and
	// webcam callbacks. Handler:
	// internal/telegram/handlers/med.go.
	NSMedia = "med"

	// NSNotify is the namespace for desktop-notification and
	// text-to-speech callbacks. Handler:
	// internal/telegram/handlers/ntf.go.
	NSNotify = "ntf"

	// NSTools is the namespace for clipboard, timezone picker,
	// disks, time sync, and Shortcuts callbacks. Handler:
	// internal/telegram/handlers/tls.go.
	NSTools = "tls"

	// NSNav is the namespace for cross-cutting navigation (back
	// to home, refresh). Handler:
	// internal/telegram/handlers/nav.go.
	NSNav = "nav"
)

// AllNamespaces is the canonical ordered list of every NS…
// constant. Used by the dispatcher's startup self-check (every
// namespace must have a registered handler) and by the keyboard
// layer's assertion tests.
var AllNamespaces = []string{
	NSSound, NSDisplay, NSPower, NSWifi, NSBT, NSBattery,
	NSSystem, NSMedia, NSNotify, NSTools, NSNav,
}
