// Package status aggregates a single one-screen dashboard
// snapshot by composing the other read-only domain packages.
//
// Used by the legacy `/status` command and the boot-ping
// message that the daemon sends to the first whitelisted user
// when it comes up. Differs from the dashboard categories in
// that it owns NO macOS CLI of its own — every value comes from
// the underlying [system], [battery], and [wifi] services.
//
// The package surface is intentionally small: one [Service]
// constructor and one [Snapshot] return type. Aggregation and
// error joining are the only logic.
package status

import (
	"context"
	"errors"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Snapshot is the aggregated cross-domain dashboard snapshot
// returned by [Service.Snapshot]. Each "happy" field is paired
// with its own *Err field so callers can render partial output
// when one source fails.
//
// Lifecycle:
//   - Constructed by Service.Snapshot each time /status runs.
//     Never cached.
//   - Read-only from the caller's perspective; the snapshot is
//     a value, not a stream.
//
// Field roles:
//   - Info / Battery / WiFi are the typed snapshots from the
//     underlying domain services. Each may be a zero value when
//     its corresponding *Err field is non-nil.
//   - InfoErr / BatteryErr / WiFiErr carry the per-source
//     failure (typically a subprocess error). Non-nil here
//     means "render the alternative 'unavailable' line for
//     this section."
type Snapshot struct {
	// Info is the macOS / hardware summary from
	// [system.Service.Info]. Zero-valued when InfoErr is
	// non-nil.
	Info system.Info

	// Battery is the live battery snapshot from
	// [battery.Service.Get]. Zero-valued when BatteryErr is
	// non-nil.
	Battery battery.Status

	// WiFi is the Wi-Fi state snapshot from [wifi.Service.Get].
	// Zero-valued when WiFiErr is non-nil.
	WiFi wifi.Info

	// BatteryErr is the per-source error from
	// [battery.Service.Get], if any. nil on success.
	BatteryErr error

	// WiFiErr is the per-source error from [wifi.Service.Get],
	// if any. nil on success.
	WiFiErr error

	// InfoErr is the per-source error from
	// [system.Service.Info], if any. nil on success.
	InfoErr error
}

// Service composes the other read-only domain services for
// dashboard aggregation. One instance per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Status.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     The composed services are themselves concurrent-safe.
//
// Field roles:
//   - sys / bat / wif are the underlying [system], [battery],
//     and [wifi] service instances. Each constructed against
//     the same [runner.Runner] passed to [New], so they share
//     the subprocess boundary and timeout policy.
type Service struct {
	// sys is the underlying [system.Service] read by Snapshot.
	sys *system.Service

	// bat is the underlying [battery.Service] read by Snapshot.
	bat *battery.Service

	// wif is the underlying [wifi.Service] read by Snapshot.
	wif *wifi.Service
}

// New returns a composed [Service] backed by r. Constructs
// fresh [system.Service], [battery.Service], and [wifi.Service]
// instances internally — callers don't pass them in. Pass
// [runner.New] in production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service {
	return &Service{
		sys: system.New(r),
		bat: battery.New(r),
		wif: wifi.New(r),
	}
}

// Snapshot reads all three composed services and returns the
// aggregated [Snapshot].
//
// Behavior:
//  1. Calls system.Info, battery.Get, and wifi.Get sequentially.
//     They run sequentially (not in parallel) because the cost
//     is dominated by subprocess startup overhead, and macOS
//     serialises most of these CLIs against shared kernel
//     locks anyway — running concurrently saves little wall
//     time and complicates context-cancellation.
//  2. Captures each per-source error in its corresponding
//     *Err field on the snapshot.
//  3. Returns the snapshot + nil error when at least one source
//     succeeded.
//  4. Returns the snapshot + a joined error (via [errors.Join]
//     of all three Err fields) ONLY when ALL three sources
//     failed — the dashboard can't render anything useful in
//     that case.
//
// Returns the snapshot in every case; the returned error is
// non-nil only on total failure.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{}
	snap.Info, snap.InfoErr = s.sys.Info(ctx)
	snap.Battery, snap.BatteryErr = s.bat.Get(ctx)
	snap.WiFi, snap.WiFiErr = s.wif.Get(ctx)

	if snap.InfoErr != nil && snap.BatteryErr != nil && snap.WiFiErr != nil {
		return snap, errors.Join(snap.InfoErr, snap.BatteryErr, snap.WiFiErr)
	}
	return snap, nil
}
