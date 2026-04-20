package handlers

import "github.com/amiwrpremium/macontrol/internal/domain/media"

// mediaSilentOpts is split out so `/screenshot` can reuse the same defaults
// as the Media → 📷 Silent shot button without depending on media.Package
// from commands.go.
func mediaSilentOpts() media.ScreenshotOpts {
	return media.ScreenshotOpts{Silent: true}
}
