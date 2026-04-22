// Package wifi controls Wi-Fi via `networksetup`, `wdutil`, and
// `networkQuality` (the built-in speed test shipped in macOS 12+).
package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Info summarizes current Wi-Fi state. SSID and the rich fields
// (BSSID, RSSI, Security, TxRateMbps, Channel) come from
// `wdutil info` (preferred, sudo) or `system_profiler SPAirPortDataType`
// (fallback, no sudo). They may be empty / zero if neither source
// could be queried.
type Info struct {
	Interface  string  // e.g. "en0"
	PowerOn    bool    //
	SSID       string  // empty if not joined or Wi-Fi off
	BSSID      string  // AP MAC, e.g. "aa:bb:cc:dd:ee:ff"
	RSSI       int     // signal strength in dBm (negative; 0 = unknown)
	Security   string  // e.g. "WPA2 Personal", "WPA/WPA2 Personal", "Open"
	TxRateMbps float64 // current PHY link rate in Mbps; 0 = unknown
	Channel    string  // e.g. "2g3/20" (wdutil) or "3 (2GHz, 20MHz)" (system_profiler)
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

// Get returns current Wi-Fi info. SSID + rich fields come from
// `wdutil info` (already in macontrol's narrow sudoers entry); on
// macOS 14.4+ `networksetup -getairportnetwork` returns "not
// associated" for non-Location-permission processes, so we don't
// use it. Falls back to `system_profiler SPAirPortDataType` (slow,
// ~2-3s) if wdutil is unauthorized.
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
		s.populateAssociation(ctx, &info)
	}
	return info, nil
}

// populateAssociation fills SSID and the rich fields. Tries wdutil
// first (fast, sudoers-authorized), then system_profiler (slow,
// unprivileged). Best-effort — leaves zero values on total failure
// so the caller can still render a useful dashboard.
func (s *Service) populateAssociation(ctx context.Context, info *Info) {
	if out, err := s.r.Sudo(ctx, "wdutil", "info"); err == nil {
		applyWdutilInfo(info, string(out))
		if info.SSID != "" {
			return
		}
	}
	if out, err := s.r.Exec(ctx, "system_profiler", "SPAirPortDataType"); err == nil {
		applySystemProfilerInfo(info, string(out))
	}
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

// applyWdutilInfo parses `sudo wdutil info` output and populates
// fields on info. wdutil emits sectioned output with `KEY : VALUE`
// lines under section banners; we only read the WIFI section.
// Fields not present in the output are left at their zero value.
func applyWdutilInfo(info *Info, out string) {
	inWifi := false
	for _, raw := range strings.Split(out, "\n") {
		// Section banner: a line of em-dashes precedes / follows the
		// section name. We just look for the section name itself.
		trim := strings.TrimSpace(raw)
		switch trim {
		case "WIFI":
			inWifi = true
			continue
		case "NETWORK", "BLUETOOTH", "AWDL", "POWER",
			"WIFI FAULTS LAST HOUR",
			"WIFI RECOVERIES LAST HOUR",
			"WIFI LINK TESTS LAST HOUR":
			inWifi = false
			continue
		}
		if !inWifi {
			continue
		}
		key, val, ok := splitKeyVal(raw)
		if !ok {
			continue
		}
		switch key {
		case "SSID":
			info.SSID = val
		case "BSSID":
			info.BSSID = val
		case "RSSI":
			// e.g. "-45 dBm"
			fields := strings.Fields(val)
			if len(fields) > 0 {
				if v, err := strconv.Atoi(fields[0]); err == nil {
					info.RSSI = v
				}
			}
		case "Security":
			info.Security = val
		case "Tx Rate":
			// e.g. "144.0 Mbps"
			info.TxRateMbps = parseMbps(val)
		case "Channel":
			info.Channel = val
		}
	}
}

// applySystemProfilerInfo parses `system_profiler SPAirPortDataType`
// output. The current network's SSID appears as a sub-heading right
// after the line `Current Network Information:`. Other fields
// (channel, security) are then nested two more indents deeper.
//
// Only sets fields we don't already have populated, so the wdutil
// pass takes precedence when both run.
func applySystemProfilerInfo(info *Info, out string) {
	lines := strings.Split(out, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.Contains(line, "Current Network Information:") {
			continue
		}
		// Expect the next non-blank line to be the SSID heading,
		// indented further than the "Current Network Information:" line
		// and ending with ':'.
		baseIndent := indentWidth(line)
		for j := i + 1; j < len(lines); j++ {
			cand := lines[j]
			if strings.TrimSpace(cand) == "" {
				continue
			}
			candIndent := indentWidth(cand)
			if candIndent <= baseIndent {
				return // moved out of the block; SSID heading not found
			}
			cand = strings.TrimSpace(cand)
			if strings.HasSuffix(cand, ":") {
				ssid := strings.TrimSuffix(cand, ":")
				if info.SSID == "" {
					info.SSID = ssid
				}
				// Walk the inner block for additional fields.
				for k := j + 1; k < len(lines); k++ {
					sub := lines[k]
					if strings.TrimSpace(sub) == "" {
						continue
					}
					if indentWidth(sub) <= candIndent {
						return
					}
					subTrim := strings.TrimSpace(sub)
					switch {
					case strings.HasPrefix(subTrim, "Channel: ") && info.Channel == "":
						info.Channel = strings.TrimPrefix(subTrim, "Channel: ")
					case strings.HasPrefix(subTrim, "Security: ") && info.Security == "":
						info.Security = strings.TrimPrefix(subTrim, "Security: ")
					case strings.HasPrefix(subTrim, "Transmit Rate: ") && info.TxRateMbps == 0:
						v, _ := strconv.ParseFloat(strings.TrimPrefix(subTrim, "Transmit Rate: "), 64)
						info.TxRateMbps = v
					case strings.HasPrefix(subTrim, "Signal / Noise: ") && info.RSSI == 0:
						// e.g. "Signal / Noise: -48 dBm / -84 dBm"
						parts := strings.Fields(strings.TrimPrefix(subTrim, "Signal / Noise: "))
						if len(parts) > 0 {
							if v, err := strconv.Atoi(parts[0]); err == nil {
								info.RSSI = v
							}
						}
					}
				}
				return
			}
			return // unexpected non-heading line
		}
	}
}

// splitKeyVal splits a wdutil "    KEY      : VALUE" line into key
// and trimmed value. Returns ok=false for lines without a ':' or
// with an empty key.
func splitKeyVal(raw string) (key, val string, ok bool) {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(raw[:idx])
	val = strings.TrimSpace(raw[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

// indentWidth returns the number of leading spaces on a line.
func indentWidth(line string) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(line)
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
