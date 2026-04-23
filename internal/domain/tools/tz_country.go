package tools

import (
	"os"
	"strings"
	"sync"
)

// zoneTabPaths lists the candidate file paths in priority order.
// macOS and Linux tzdata installs both ship one of these files, but
// the exact layout varies by version. Format is tab-separated:
//
//	#country-codes	coordinates	TZ	[comments]
//	IR	+3540+05131	Asia/Tehran
//	CA,US	+340308-1181434	America/Los_Angeles	Pacific
//
// `zone1970.tab` is the modern file (multi-country aware); `zone.tab`
// is the older single-country variant with the same column layout.
// Multi-country rows take the first listed code.
var zoneTabPaths = []string{
	"/usr/share/zoneinfo/zone1970.tab",
	"/usr/share/zoneinfo/zone.tab",
	"/var/db/timezone/zoneinfo/zone1970.tab",
	"/var/db/timezone/zoneinfo/zone.tab",
}

var (
	tzCountryOnce sync.Once
	tzCountryMap  map[string]string // iana name → iso2 country code
)

// LookupCountry returns the ISO 3166-1 alpha-2 country code for an
// IANA timezone, parsed once from the system's zoneinfo .tab file.
// Returns ("", false) for timezones not in the table (Antarctica/*,
// GMT, UTC, legacy macOS-only zones) or if no .tab file is available.
func LookupCountry(tz string) (iso2 string, ok bool) {
	tzCountryOnce.Do(loadZoneTab)
	if tzCountryMap == nil {
		return "", false
	}
	code, ok := tzCountryMap[tz]
	return code, ok
}

func loadZoneTab() {
	for _, path := range zoneTabPaths {
		if m := parseZoneTab(path); len(m) > 0 {
			tzCountryMap = m
			return
		}
	}
	// All candidates missing/unparseable → flags disabled globally.
}

func parseZoneTab(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
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
	return m
}
