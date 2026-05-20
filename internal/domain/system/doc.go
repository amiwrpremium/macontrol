// Package system exposes read-only macOS inspection: OS version and build,
// hardware model, chip, memory, CPU load and top processes, thermal
// pressure, and approximate temperatures.
//
// Every [Service] method is best-effort — missing data is reported as zero
// values plus a Raw string the caller can fall back to, rather than as an
// error. The package shells out to the standard macOS CLIs (`sw_vers`,
// `sysctl`, `uptime`, `top`, `ps`, `memory_pressure`, `system_profiler`)
// and, for thermal pressure, uses the narrow `sudo pmset`/`sudo powermetrics`
// entries configured by `macontrol setup`.
//
// `smctemp` (Homebrew formula) is optional — Apple Silicon Mac temperature
// readings fall back to "unavailable" when it isn't installed.
package system
