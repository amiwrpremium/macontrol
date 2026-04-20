// Package bluetooth wraps the `blueutil` brew formula to expose power and
// device control. Falls back to `system_profiler SPBluetoothDataType` for
// read-only status if blueutil is missing.
package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Device is a paired/known Bluetooth device.
type Device struct {
	Address   string `json:"address"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Paired    bool   `json:"paired"`
	Favourite bool   `json:"favourite"`
}

// State is a snapshot of the Bluetooth radio.
type State struct {
	PowerOn bool
}

// Service controls Bluetooth. blueutil is required for toggling; read-only
// status works without it.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Get returns current radio state.
func (s *Service) Get(ctx context.Context) (State, error) {
	out, err := s.r.Exec(ctx, "blueutil", "-p")
	if err != nil {
		return State{}, err
	}
	return State{PowerOn: strings.TrimSpace(string(out)) == "1"}, nil
}

// SetPower turns the Bluetooth radio on or off.
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

// Toggle flips Bluetooth power.
func (s *Service) Toggle(ctx context.Context) (State, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return State{}, err
	}
	return s.SetPower(ctx, !cur.PowerOn)
}

// Paired lists paired devices (includes disconnected ones).
func (s *Service) Paired(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "--paired")
}

// Connected lists currently-connected devices.
func (s *Service) Connected(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "--connected")
}

// Connect establishes a connection to a paired device by MAC.
func (s *Service) Connect(ctx context.Context, address string) error {
	_, err := s.r.Exec(ctx, "blueutil", "--connect", address)
	return err
}

// Disconnect drops the connection to a device by MAC.
func (s *Service) Disconnect(ctx context.Context, address string) error {
	_, err := s.r.Exec(ctx, "blueutil", "--disconnect", address)
	return err
}

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
