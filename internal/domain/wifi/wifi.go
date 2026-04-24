// Package wifi controls the Mac's Wi-Fi radio and reads its
// current state.
//
// Three macOS CLIs back the package:
//
//   - networksetup — for power on/off, interface discovery, joining
//     networks, and DNS server configuration. Available since
//     macOS 10.x; the only one that doesn't need sudo.
//   - wdutil       — for SSID, BSSID, RSSI, security, channel, and
//     tx-rate. Requires the narrow sudoers entry. Preferred over
//     networksetup because Apple broke
//     `networksetup -getairportnetwork` in macOS 14.4 (it now
//     reports "not associated" for processes without Location
//     permission, which a daemon can't easily acquire).
//   - networkQuality — for the built-in speed test. Shipped in
//     macOS 12+; gated behind [capability.Features.NetworkQuality].
//
// Public surface:
//
//   - [Info] is the read-side dashboard snapshot.
//   - [SpeedResult] is the speed-test result snapshot.
//   - [Service] groups every operation with the [runner.Runner]
//     dependency injected at construction.
//
// Parsing is split into helpers ([applyWdutilInfo],
// [applySystemProfilerInfo]) so each parser can be tested in
// isolation against captured output samples.
package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Info is the read-side snapshot of current Wi-Fi state. Returned
// by [Service.Get] (or by any operation that calls Get internally
// to refresh after a change, e.g. [Service.SetPower],
// [Service.Toggle], [Service.Join]).
//
// Lifecycle:
//   - Constructed by Service.Get on every call. Never cached;
//     handlers ask for a fresh Info on every refresh tap.
//
// Field roles:
//   - Interface is the kernel-visible Wi-Fi device name (typically
//     "en0"; sometimes "en1" on Macs with multiple network
//     ports). Discovered via [Service.Interface].
//   - PowerOn is the radio toggle state read from
//     networksetup -getairportpower.
//   - SSID + the rich link fields (BSSID, RSSI, Security,
//     TxRateMbps, Channel) come from wdutil info (sudoers-
//     authorised) with a system_profiler fallback. Any of these
//     may be empty / zero when neither source could populate
//     them — typical when Wi-Fi is off, the daemon's sudoers
//     entry is missing, or the host is mid-handover. Handlers
//     must tolerate any subset being unset.
type Info struct {
	// Interface is the kernel-visible Wi-Fi device name,
	// typically "en0".
	Interface string

	// PowerOn is true when the radio is on, read from
	// networksetup -getairportpower.
	PowerOn bool

	// SSID is the joined network name; empty when not associated
	// or when Wi-Fi is off.
	SSID string

	// BSSID is the access-point MAC address, e.g.
	// "aa:bb:cc:dd:ee:ff". Empty when neither source populated
	// it.
	BSSID string

	// RSSI is the signal strength in dBm. Healthy links land in
	// the -40 to -70 range. Zero means unknown — the field is
	// not reported, NOT 0 dBm.
	RSSI int

	// Security is the authentication mode string verbatim from
	// the source CLI, e.g. "WPA2 Personal" / "WPA/WPA2 Personal"
	// / "Open".
	Security string

	// TxRateMbps is the current physical link rate in megabits
	// per second. Zero means unknown.
	TxRateMbps float64

	// Channel is the operating channel in the format produced by
	// the source CLI: "2g3/20" from wdutil (band/number/width)
	// or "3 (2GHz, 20MHz)" from system_profiler.
	Channel string
}

// SpeedResult is one round of [Service.Speedtest] output.
//
// Lifecycle:
//   - Constructed once per Service.Speedtest call. The Raw field
//     is held verbatim so handlers can dump it as a fallback if
//     the parsed Mbps fields are zero (parser missed a field).
//
// Field roles:
//   - DownloadMbps / UploadMbps are the parsed capacity numbers
//     in megabits per second. Zero means parser miss, not zero
//     bandwidth.
//   - Raw is the full unmodified `networkQuality -v` output.
type SpeedResult struct {
	// DownloadMbps is the downlink capacity in megabits per
	// second.
	DownloadMbps float64

	// UploadMbps is the uplink capacity in megabits per second.
	UploadMbps float64

	// Raw is the full verbatim output of `networkQuality -v` for
	// fallback display when parsing misses a field.
	Raw string
}

// Service is the Wi-Fi control surface. One instance per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.WiFi.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations
//     (the [runner.Runner] is itself concurrent-safe).
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] (with pre-registered rules)
// in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Interface returns the first Wi-Fi hardware port discovered by
// `networksetup -listallhardwareports`, typically "en0".
//
// Behavior:
//   - Walks the hardware-ports listing line-by-line, tracking
//     whether the most recently seen "Hardware Port:" line was
//     "Wi-Fi" or its legacy "AirPort" name.
//   - Returns the first matching "Device:" line that follows a
//     Wi-Fi port banner.
//   - Returns "no Wi-Fi hardware port found" when no Wi-Fi port
//     is present (Mac mini / Mac Pro variants without Wi-Fi).
//
// Safe to cache the result for the daemon's lifetime — hardware
// ports don't appear or disappear without a reboot. macontrol
// re-queries on every call instead, which is cheap (~50ms
// subprocess).
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

// Get returns the current [Info] snapshot.
//
// Behavior:
//  1. Resolves the interface name via [Service.Interface]. Errors
//     short-circuit.
//  2. Reads power state via networksetup -getairportpower; sets
//     PowerOn = strings.Contains(stdout, ": On"). Errors return
//     a partial Info with Interface populated.
//  3. When PowerOn is true, calls [Service.populateAssociation]
//     to fill SSID and the rich fields. Best-effort — population
//     errors are swallowed so the dashboard always renders.
//  4. When PowerOn is false, returns the partial Info with no
//     association fields set.
//
// Returns the populated Info on success; on early-stage failure
// returns a zero-or-partial Info plus the underlying error.
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

// populateAssociation fills SSID and the rich link fields on
// info via the wdutil-then-system_profiler waterfall.
//
// Routing rules (first match wins):
//  1. `sudo wdutil info` succeeds AND populates a non-empty SSID
//     → return; no system_profiler call needed.
//  2. wdutil fails (sudoers entry missing, kernel hiccup) OR
//     succeeds with an empty SSID → fall through to
//     `system_profiler SPAirPortDataType`. Slow (~2-3s) and
//     unprivileged.
//  3. system_profiler also fails → leave the existing fields at
//     zero; caller still sees Interface and PowerOn.
//
// All errors are swallowed by design — the dashboard prefers
// partial information over an error toast that hides
// already-known fields.
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

// SetPower turns the Wi-Fi radio on or off and returns a fresh
// [Info] snapshot reflecting the new state.
//
// Behavior:
//   - Resolves the interface, then calls
//     `networksetup -setairportpower <iface> on|off`.
//   - On success, follows up with [Service.Get] so the caller
//     always sees the post-change state.
//
// Returns the post-change Info on success; (zero Info, err) on
// any subprocess failure.
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

// Toggle flips the Wi-Fi power state from whatever it currently
// is. Composes [Service.Get] with [Service.SetPower] so a single
// "Toggle" tap in the dashboard does the right thing without
// the handler needing to read state first.
//
// Returns the post-toggle [Info] snapshot.
func (s *Service) Toggle(ctx context.Context) (Info, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return Info{}, err
	}
	return s.SetPower(ctx, !cur.PowerOn)
}

// Join associates with ssid using password. Returns a fresh Info
// snapshot reflecting the new (or unchanged-on-failure)
// association.
//
// Behavior:
//   - When password is empty, omits the password argument to
//     networksetup so the call works for open networks. Wi-Fi
//     join flows pass the literal "-" for open networks at the
//     UX layer; the flow then converts it to "" before calling
//     here.
//   - When password is non-empty, appends it as the third
//     positional arg to `networksetup -setairportnetwork <iface>
//     <ssid> <password>`.
//   - On success, follows with [Service.Get] for the post-join
//     snapshot.
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

// SetDNS sets the DNS resolvers for the "Wi-Fi" service.
//
// Behavior:
//   - When servers is nil or empty, sends the literal "Empty"
//     argument that networksetup interprets as "reset to DHCP".
//   - When servers is non-empty, joins them as space-separated
//     positional args.
//
// Returns the underlying networksetup error, or nil on success.
// Does not refresh the Info snapshot — DNS state isn't reflected
// on Info anyway, and the handler renders a separate
// "DNS updated → <preset>" status line.
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

// Diag runs `sudo wdutil info` and returns the verbatim output
// for the Wi-Fi diagnostics drill-down. Same source as
// [populateAssociation] but unparsed — handlers display the raw
// dump in a code block so the user sees BSSID, channel, noise,
// per-stream RSSI, etc.
//
// Returns the raw stdout and any subprocess error.
func (s *Service) Diag(ctx context.Context) (string, error) {
	out, err := s.r.Sudo(ctx, "wdutil", "info")
	return string(out), err
}

// Speedtest runs `networkQuality -v` and parses its output into
// a [SpeedResult].
//
// Behavior:
//   - Wraps ctx with a 60-second timeout because Apple's
//     measurement runs upload + download + RPM end-to-end and
//     reliably takes 15-25s (longer on slow connections), which
//     overruns the default 15-s [runner.DefaultTimeout].
//   - Parses lines starting with "Downlink capacity:" /
//     "Uplink capacity:" via [parseMbps]; missed fields stay at
//     zero and the caller can fall back to displaying SpeedResult.Raw.
//
// Returns the populated SpeedResult on success; (zero
// SpeedResult, err) on any subprocess failure or context expiry.
func (s *Service) Speedtest(ctx context.Context) (SpeedResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
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
// the relevant fields on info. wdutil emits sectioned output;
// only the WIFI section is consumed here — other sections
// (NETWORK, BLUETOOTH, AWDL, POWER, …) are explicitly ignored.
//
// Behavior:
//   - Walks the output line by line, tracking inWifi state via
//     section banner detection.
//   - For each KEY: VALUE line in the WIFI section, switches on
//     KEY and writes the parsed value into info.
//   - Fields not present in the output are left at their
//     zero value — partial parses are normal.
//   - The list of "section enders" (NETWORK, BLUETOOTH, …) is
//     hand-maintained; a future macOS release that adds a new
//     sibling section will be classified as still-in-WIFI until
//     the list is updated, which would cause garbage from that
//     section to leak into info. See the smells list.
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

// applySystemProfilerInfo parses
// `system_profiler SPAirPortDataType` output as a fallback
// source for SSID + rich link fields. The current network's SSID
// appears as a sub-heading immediately under the
// "Current Network Information:" line; channel, security, etc.
// are nested two more indents deeper.
//
// Behavior:
//   - Linear scan looking for "Current Network Information:".
//   - On match, walks subsequent lines indent-aware to find the
//     SSID heading (a line ending with ':' that's more indented
//     than the current-network line).
//   - Walks the inner block looking for Channel: / Security: /
//     Transmit Rate: / Signal / Noise: lines, each populating
//     its corresponding Info field IF the field is still at its
//     zero value (so the wdutil pass takes precedence when both
//     ran).
//   - Returns silently when the indent structure breaks or when
//     no SSID heading is found.
//
// The indent-based parsing is fragile but matches Apple's
// long-stable formatting; see the smells list for the brittleness
// note.
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

// splitKeyVal splits a wdutil "    KEY      : VALUE" line into
// key + value, both whitespace-trimmed.
//
// Behavior:
//   - Returns ok=false when raw has no ':' separator.
//   - Returns ok=false when the trimmed key is empty (defends
//     against lines that are just ": foo" garbage).
//   - Splits on the FIRST ':' only, so values containing ':'
//     (e.g. BSSIDs aa:bb:cc:dd:ee:ff) survive intact.
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

// indentWidth returns the number of leading whitespace runes
// (spaces or tabs) on line, used by [applySystemProfilerInfo]'s
// indent-aware block detection. Returns len(line) for an
// all-whitespace line so it cannot be confused with a less-
// indented heading.
func indentWidth(line string) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(line)
}

// parseMbps extracts the numeric component preceding the literal
// "Mbps" token in line. Returns 0 when no "Mbps" token is found
// or when the preceding token is not a parseable float.
//
// Used for both wdutil's "Tx Rate: 144.0 Mbps" lines and
// networkQuality's "Downlink capacity: 850.123 Mbps" lines.
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
