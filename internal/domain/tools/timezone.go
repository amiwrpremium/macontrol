package tools

import (
	"context"
	"strings"
)

// TimezoneCurrent returns the currently-set timezone, e.g. "Europe/Istanbul".
func (s *Service) TimezoneCurrent(ctx context.Context) (string, error) {
	out, err := s.r.Sudo(ctx, "systemsetup", "-gettimezone")
	if err != nil {
		return "", err
	}
	// Output looks like: "Time Zone: Europe/Istanbul"
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, ":"); idx >= 0 {
		return strings.TrimSpace(line[idx+1:]), nil
	}
	return line, nil
}

// TimezoneList returns the list of valid timezones reported by macOS.
func (s *Service) TimezoneList(ctx context.Context) ([]string, error) {
	out, err := s.r.Sudo(ctx, "systemsetup", "-listtimezones")
	if err != nil {
		return nil, err
	}
	zones := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Time Zones:") {
			continue
		}
		zones = append(zones, line)
	}
	return zones, nil
}

// TimezoneSet sets the system timezone. Requires the sudoers entry.
func (s *Service) TimezoneSet(ctx context.Context, tz string) error {
	_, err := s.r.Sudo(ctx, "systemsetup", "-settimezone", tz)
	return err
}

// TimeSync forces an NTP sync. Best-effort.
func (s *Service) TimeSync(ctx context.Context) error {
	_, err := s.r.Sudo(ctx, "sntp", "-sS", "time.apple.com")
	return err
}
