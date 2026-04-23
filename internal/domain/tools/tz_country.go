package tools

import (
	"os"
	"strings"
	"sync"
)

// zoneTabPath is the standard tzdata file shipped with macOS that
// maps IANA timezone names to ISO 3166-1 alpha-2 country codes.
// Format (tab-separated):
//
//	#country-codes	coordinates	TZ	[comments]
//	IR	+3540+05131	Asia/Tehran
//	CA,US	+340308-1181434	America/Los_Angeles	Pacific
//
// Multi-country rows take the first listed code.
const zoneTabPath = "/usr/share/zoneinfo/zone1970.tab"

var (
	tzCountryOnce sync.Once
	tzCountryMap  map[string]string // iana name → iso2 country code
)

// LookupCountry returns the ISO 3166-1 alpha-2 country code for an
// IANA timezone, parsed once from /usr/share/zoneinfo/zone1970.tab.
// Returns ("", false) for timezones not in the table (Antarctica/*,
// GMT, UTC, legacy macOS-only zones) or if the file is missing /
// unparseable.
func LookupCountry(tz string) (iso2 string, ok bool) {
	tzCountryOnce.Do(loadZoneTab)
	if tzCountryMap == nil {
		return "", false
	}
	code, ok := tzCountryMap[tz]
	return code, ok
}

func loadZoneTab() {
	data, err := os.ReadFile(zoneTabPath)
	if err != nil {
		return // graceful: lookups will return ("",false) forever
	}
	m := make(map[string]string, 400)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		// fields[0] = "IR" or "CA,US"; take the first comma-separated.
		codes := strings.Split(fields[0], ",")
		if len(codes) == 0 || len(codes[0]) != 2 {
			continue
		}
		// fields[2] = IANA timezone name.
		m[fields[2]] = codes[0]
	}
	if len(m) > 0 {
		tzCountryMap = m
	}
}
