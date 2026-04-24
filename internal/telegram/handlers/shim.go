package handlers

import "github.com/amiwrpremium/macontrol/internal/domain/media"

// mediaSilentOpts returns the [media.ScreenshotOpts] used by
// the "silent screenshot" callers — both the `/screenshot`
// slash command in commands.go and the Media → 📷 Silent
// dashboard button.
//
// Lives in this thin shim file so the two callers can reuse
// the same defaults without commands.go having to import the
// [media] package directly. (commands.go already pulls in
// several domain packages; isolating this one helper keeps
// the dependency graph slightly cleaner.)
//
// Returns ScreenshotOpts{Silent: true} — every other field
// stays at its zero value (Display 0 means "every attached
// display", Delay 0 means "fire immediately").
func mediaSilentOpts() media.ScreenshotOpts {
	return media.ScreenshotOpts{Silent: true}
}
