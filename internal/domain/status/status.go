// Package status aggregates a one-screen dashboard snapshot from the other
// domain packages. Used by /status and the boot-ping message.
package status

import (
	"context"
	"errors"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Snapshot is the aggregated report.
type Snapshot struct {
	Info       system.Info
	Battery    battery.Status
	WiFi       wifi.Info
	BatteryErr error
	WiFiErr    error
	InfoErr    error
}

// Service composes the other read-only services.
type Service struct {
	sys *system.Service
	bat *battery.Service
	wif *wifi.Service
}

// New returns a composed Service.
func New(r runner.Runner) *Service {
	return &Service{
		sys: system.New(r),
		bat: battery.New(r),
		wif: wifi.New(r),
	}
}

// Snapshot reads all three concurrent-safe underlying services sequentially
// (the cost is dominated by subprocess startup, not wall-time). Sub-errors
// are captured on the Snapshot — the call returns nil unless *nothing*
// worked.
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
