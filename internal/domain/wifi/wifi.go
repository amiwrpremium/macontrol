// Package wifi controls Wi-Fi via `networksetup`, `wdutil`, and
// `networkQuality` (the built-in speed test shipped in macOS 12+).
package wifi

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Info summarizes current Wi-Fi state.
type Info struct {
	Interface string // e.g. "en0"
	PowerOn   bool
	SSID      string // empty if not joined or Wi-Fi off
}

// SpeedResult carries a round of `networkQuality`.
type SpeedResult struct {
	DownloadMbps float64
	UploadMbps   float64
	Raw          string // full raw output (we display it verbatim for now)
}

// Service controls Wi-Fi.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Interface returns the first Wi-Fi hardware port (usually "en0").
// It is safe to cache the result for the lifetime of the daemon.
func (s *Service) Interface(ctx context.Context) (string, error) {
	out, err := s.r.Exec(ctx, "networksetup", "-listallhardwareports")
	if err != nil {
		return "", err
	}
	var lastWiFi bool
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Hardware Port:"):
			name := strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
			lastWiFi = (name == "Wi-Fi" || name == "AirPort")
		case lastWiFi && strings.HasPrefix(line, "Device:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "Device:")), nil
		}
	}
	return "", fmt.Errorf("no Wi-Fi hardware port found")
}

// Get returns current Wi-Fi info.
func (s *Service) Get(ctx context.Context) (Info, error) {
	iface, err := s.Interface(ctx)
	if err != nil {
		return Info{}, err
	}
	info := Info{Interface: iface}

	powerOut, err := s.r.Exec(ctx, "networksetup", "-getairportpower", iface)
	if err != nil {
		return info, err
	}
	info.PowerOn = strings.Contains(string(powerOut), ": On")

	if info.PowerOn {
		ssidOut, err := s.r.Exec(ctx, "networksetup", "-getairportnetwork", iface)
		if err == nil {
			info.SSID = parseSSID(string(ssidOut))
		}
	}
	return info, nil
}

// SetPower turns Wi-Fi on or off.
func (s *Service) SetPower(ctx context.Context, on bool) (Info, error) {
	iface, err := s.Interface(ctx)
	if err != nil {
		return Info{}, err
	}
	state := "off"
	if on {
		state = "on"
	}
	_, err = s.r.Exec(ctx, "networksetup", "-setairportpower", iface, state)
	if err != nil {
		return Info{}, err
	}
	return s.Get(ctx)
}

// Toggle flips Wi-Fi power.
func (s *Service) Toggle(ctx context.Context) (Info, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return Info{}, err
	}
	return s.SetPower(ctx, !cur.PowerOn)
}

// Join associates with an SSID using the provided password. Returns the
// new Info snapshot.
func (s *Service) Join(ctx context.Context, ssid, password string) (Info, error) {
	iface, err := s.Interface(ctx)
	if err != nil {
		return Info{}, err
	}
	args := []string{"-setairportnetwork", iface, ssid}
	if password != "" {
		args = append(args, password)
	}
	_, err = s.r.Exec(ctx, "networksetup", args...)
	if err != nil {
		return Info{}, err
	}
	return s.Get(ctx)
}

// SetDNS sets DNS servers on the Wi-Fi service. Pass nil to reset to DHCP.
func (s *Service) SetDNS(ctx context.Context, servers []string) error {
	args := []string{"-setdnsservers", "Wi-Fi"}
	if len(servers) == 0 {
		args = append(args, "Empty")
	} else {
		args = append(args, servers...)
	}
	_, err := s.r.Exec(ctx, "networksetup", args...)
	return err
}

// Diag runs `sudo wdutil info` for deeper diagnostics (bssid, channel, noise).
func (s *Service) Diag(ctx context.Context) (string, error) {
	out, err := s.r.Sudo(ctx, "wdutil", "info")
	return string(out), err
}

// Speedtest runs the built-in `networkQuality` tool.
func (s *Service) Speedtest(ctx context.Context) (SpeedResult, error) {
	out, err := s.r.Exec(ctx, "networkQuality", "-v")
	if err != nil {
		return SpeedResult{}, err
	}
	result := SpeedResult{Raw: string(out)}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Typical lines: "Downlink capacity: 850.123 Mbps"
		if strings.HasPrefix(line, "Downlink capacity:") {
			result.DownloadMbps = parseMbps(line)
		}
		if strings.HasPrefix(line, "Uplink capacity:") {
			result.UploadMbps = parseMbps(line)
		}
	}
	return result, nil
}

func parseSSID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Current Wi-Fi Network:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Current Wi-Fi Network:"))
		}
		if strings.Contains(line, "not associated") || strings.Contains(line, "Not associated") {
			return ""
		}
	}
	return ""
}

func parseMbps(line string) float64 {
	parts := strings.Fields(line)
	for i, p := range parts {
		if p == "Mbps" && i > 0 {
			var v float64
			_, _ = fmt.Sscanf(parts[i-1], "%f", &v)
			return v
		}
	}
	return 0
}
