// Package bluetooth wraps the `blueutil` brew formula to expose
// Bluetooth radio + paired-device control.
//
// All operations require the `blueutil` brew formula
// (`brew install blueutil`). Without it, every method fails
// with the runner's "exec: 'blueutil': not found" error and the
// dashboard surfaces that as the "install blueutil" hint.
//
// The package doc string used to claim a `system_profiler
// SPBluetoothDataType` fallback for read-only status; that
// fallback is NOT actually implemented (see the smells list).
//
// Public surface:
//
//   - [State] — the read-side power snapshot.
//   - [Device] — one paired or connected peripheral.
//   - [Service] — the per-process control surface; one instance
//     on bot.Deps.Services.Bluetooth.
package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Device is one paired or known Bluetooth peripheral as
// reported by `blueutil --paired --format json` or
// `blueutil --connected --format json`. The struct field tags
// match blueutil's JSON output exactly; do not rename without
// also adjusting the Unmarshal call.
//
// Field roles:
//   - Address is the MAC address. Used as the lookup key by
//     [Service.Connect] / [Service.Disconnect].
//   - Name is the user-friendly device name (e.g. "AirPods Pro").
//   - Connected is true when the device is currently active.
//     Combined with Paired this can be {true,true} (paired and
//     in use), {false,true} (paired but idle), or {true,false}
//     (connected but not paired — only valid for transient
//     devices like incoming-pairing prompts).
//   - Paired is true when the device is in the user's paired
//     list. Always true for [Service.Paired] results.
//   - Favourite is true when macOS marks this device as a
//     "Favourites" entry; informational only — macontrol doesn't
//     act on it.
type Device struct {
	// Address is the MAC address, e.g. "aa:bb:cc:dd:ee:ff".
	Address string `json:"address"`

	// Name is the user-friendly device name.
	Name string `json:"name"`

	// Connected is true when the device is currently active.
	Connected bool `json:"connected"`

	// Paired is true when the device is in the user's paired
	// list.
	Paired bool `json:"paired"`

	// Favourite is true when macOS marks this device as a
	// "Favourites" entry.
	Favourite bool `json:"favourite"`
}

// State is the read-side snapshot of the Bluetooth radio
// power. Returned by [Service.Get] and by every write method.
//
// Lifecycle:
//   - Constructed by Service.Get (or Service.SetPower) on every
//     call. Never cached.
//
// Field roles:
//   - PowerOn is true when blueutil reports the radio as on
//     (`blueutil -p` returns "1"). False otherwise.
type State struct {
	// PowerOn is true when the Bluetooth radio is on.
	PowerOn bool
}

// Service is the Bluetooth control surface. One instance per
// process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Bluetooth.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     blueutil itself serialises against the macOS Bluetooth
//     subsystem so concurrent Connect/Disconnect calls land
//     in submission order.
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Get returns the current radio [State] via `blueutil -p`.
//
// Behavior:
//   - blueutil -p prints "1" for on, "0" for off followed by
//     a newline. The trim+compare picks up either.
//   - Returns the runner error verbatim on subprocess failure
//     (typical: "exec: 'blueutil': not found" when the brew
//     formula isn't installed).
func (s *Service) Get(ctx context.Context) (State, error) {
	out, err := s.r.Exec(ctx, "blueutil", "-p")
	if err != nil {
		return State{}, err
	}
	return State{PowerOn: strings.TrimSpace(string(out)) == "1"}, nil
}

// SetPower turns the Bluetooth radio on or off.
//
// Behavior:
//   - Maps on=true → "1", on=false → "0".
//   - Runs `blueutil --power <0|1>`.
//   - On success, returns State{PowerOn: on} WITHOUT a follow-
//     up Get — saves a subprocess call. The reported state
//     might lag the actual radio state by ~100ms while macOS
//     transitions; for the dashboard this is invisible.
func (s *Service) SetPower(ctx context.Context, on bool) (State, error) {
	val := "0"
	if on {
		val = "1"
	}
	_, err := s.r.Exec(ctx, "blueutil", "--power", val)
	if err != nil {
		return State{}, err
	}
	return State{PowerOn: on}, nil
}

// Toggle flips the Bluetooth power state. Composes
// [Service.Get] with [Service.SetPower]; the read-modify-write
// is non-atomic but the Telegram dispatcher serialises updates
// per chat so the race isn't observable in practice.
//
// Returns the post-toggle [State].
func (s *Service) Toggle(ctx context.Context) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.SetPower(ctx, !cur.PowerOn)
}

// Paired lists every paired device, including disconnected
// ones. Result includes Connected=true entries when applicable.
// Delegates to [Service.listDevices] with the `--paired` flag.
func (s *Service) Paired(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "--paired")
}

// Connected lists currently-connected devices only. Subset of
// [Service.Paired] for the Connected==true entries. Delegates
// to [Service.listDevices] with the `--connected` flag.
func (s *Service) Connected(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "--connected")
}

// Connect establishes a connection to a paired device by MAC
// address.
//
// Behavior:
//   - Runs `blueutil --connect <address>`.
//   - Returns the runner error verbatim on failure. blueutil
//     emits actionable messages for the typical failures
//     ("Device not found", "Device not paired", "Already
//     connected").
func (s *Service) Connect(ctx context.Context, address string) error {
	_, err := s.r.Exec(ctx, "blueutil", "--connect", address)
	return err
}

// Disconnect drops the connection to a device by MAC address.
//
// Behavior:
//   - Runs `blueutil --disconnect <address>`.
//   - Returns the runner error verbatim on failure. Disconnecting
//     an already-disconnected device exits zero (no-op).
func (s *Service) Disconnect(ctx context.Context, address string) error {
	_, err := s.r.Exec(ctx, "blueutil", "--disconnect", address)
	return err
}

// listDevices is the shared implementation behind
// [Service.Paired] and [Service.Connected]. Runs
// `blueutil <flag> --format json` and unmarshals into
// []Device.
//
// Behavior:
//   - blueutil's JSON output is a top-level array of device
//     objects matching the [Device] field tags.
//   - Returns ("parse blueutil json: %w") wrapping the
//     json.Unmarshal error on parse failure (rare; blueutil's
//     output is stable).
//   - Returns the runner error verbatim on subprocess failure.
func (s *Service) listDevices(ctx context.Context, flag string) ([]Device, error) {
	out, err := s.r.Exec(ctx, "blueutil", flag, "--format", "json")
	if err != nil {
		return nil, err
	}
	var devs []Device
	if err := json.Unmarshal(out, &devs); err != nil {
		return nil, fmt.Errorf("parse blueutil json: %w", err)
	}
	return devs, nil
}
