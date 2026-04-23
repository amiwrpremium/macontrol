// Package tools groups small utility capabilities that don't warrant
// their own domain package: clipboard read/write, timezone query and
// set, mounted-volume listing and ejection, and the user-Shortcuts CLI
// runner.
//
// All functionality is exposed through a single [Service] that wraps
// an [internal/runner.Runner] — timezone setters go through `sudo
// systemsetup` (requires the narrow sudoers entry), clipboard and
// shortcut operations run as the caller. The Shortcuts CLI requires
// macOS 13+.
//
// [LookupCountry] is a helper used by the Telegram keyboard layer to
// attach flag emoji to timezone labels; it parses the system zoneinfo
// `zone1970.tab` once on first call.
package tools
