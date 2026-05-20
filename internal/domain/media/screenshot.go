// Package media captures still images and recordings of the Mac:
// screenshots via `screencapture`, screen recordings via the same
// CLI's record mode, and webcam single-frame photos via the
// optional `imagesnap` brew formula.
//
// Lifecycle ownership: every operation in this package writes to
// a fresh temp file and returns the path. The CALLER owns the
// returned path and is responsible for deleting it after upload
// — typically via `defer os.Remove(path)` paired with the
// upload-and-stream-it-up pattern in
// internal/telegram/handlers/handler.go's [Reply.SendPhoto] /
// [Reply.SendVideo] (which already do the cleanup).
//
// Public surface:
//
//   - [Service] — the per-process control surface; one instance
//     on bot.Deps.Services.Media.
//   - [ScreenshotOpts] — the screencapture-flag bag.
//   - [Service.Screenshot] (this file), [Service.Record]
//     (record.go), [Service.WebcamPhoto] (photo.go).
//
// Permissions: `screencapture` requires Screen Recording TCC
// permission; webcam uses Camera permission. macontrol does not
// pre-flight either — the caller will see the typical macOS
// "Process Failed" dialog or an empty file on first use until
// the user grants the permission.
package media

import (
	"context"
	"os"
	"strconv"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service is the media-capture control surface. One instance
// per process.
//
// Lifecycle:
//   - Constructed once at daemon startup via [New], stored on
//     bot.Deps.Services.Media.
//
// Concurrency:
//   - Stateless across calls; safe for concurrent invocations.
//     Each capture writes to its own temp file so concurrent
//     callers do not collide.
//
// Field roles:
//   - r is the subprocess boundary; every method shells out
//     through it.
type Service struct {
	// r is the [runner.Runner] every method shells out through.
	r runner.Runner
}

// New returns a [Service] backed by r. Pass [runner.New] in
// production; pass [runner.NewFake] in tests.
func New(r runner.Runner) *Service { return &Service{r: r} }

// ScreenshotOpts tunes the `screencapture` invocation made by
// [Service.Screenshot]. Each field corresponds to a screencapture
// CLI flag.
//
// Lifecycle:
//   - Constructed by the handler from the user's tap (which
//     dashboard variant they chose), passed to Service.Screenshot,
//     then discarded.
//
// Field roles:
//   - Display selects which display to capture. 0 means "every
//     attached display, composited side-by-side" — the
//     screencapture default. 1, 2, … select a specific display
//     by macOS's 1-indexed display ordering.
//   - Silent maps to the `-x` flag, suppressing the shutter
//     sound. Useful for "stealth" captures.
//   - Delay is the countdown in seconds before the capture
//     fires (`-T`). Zero means capture immediately. Non-zero
//     blocks the call for the duration of the countdown.
type ScreenshotOpts struct {
	// Display selects which display to capture: 0 for every
	// attached display composited together, 1/2/… for a
	// specific one (1-indexed).
	Display int

	// Silent suppresses the shutter sound by passing `-x` to
	// screencapture.
	Silent bool

	// Delay is the countdown in seconds before the capture
	// fires (`-T`). Zero means capture immediately.
	Delay int
}

// Screenshot captures the screen to a fresh temp PNG file. The
// returned path is OWNED BY THE CALLER and must be removed after
// use.
//
// Behavior:
//  1. Allocates a temp file via [tempPath] (under os.TempDir,
//     pattern "macontrol-screenshot-*.png"). Returns the
//     tempPath error if temp-file creation fails.
//  2. Builds the screencapture argument list from opts:
//     `-x` for Silent, `-T <secs>` for Delay, `-D <n>` for a
//     specific Display.
//  3. Runs `screencapture [args…] <path>`. On subprocess
//     failure, deletes the (likely empty) temp file via
//     os.Remove and returns ("", err).
//
// Returns the path on success; ("", err) on any failure (with
// the temp file already cleaned up).
//
// Caller responsibilities:
//   - Delete the returned path after upload (the
//     [Reply.SendPhoto] helper does this automatically).
//   - Grant Screen Recording TCC permission to macontrol
//     before the first call, otherwise screencapture writes a
//     blank image and silently exits zero on some macOS
//     releases.
func (s *Service) Screenshot(ctx context.Context, opts ScreenshotOpts) (string, error) {
	path, err := tempPath("macontrol-screenshot-*.png")
	if err != nil {
		return "", err
	}
	args := []string{}
	if opts.Silent {
		args = append(args, "-x")
	}
	if opts.Delay > 0 {
		args = append(args, "-T", strconv.Itoa(opts.Delay))
	}
	if opts.Display > 0 {
		args = append(args, "-D", strconv.Itoa(opts.Display))
	}
	args = append(args, path)

	if _, err := s.r.Exec(ctx, "screencapture", args...); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// tempPath allocates a fresh empty file under os.TempDir matching
// pattern (which must include "*" to act as the suffix
// placeholder per [os.CreateTemp]'s contract) and returns its
// path.
//
// Behavior:
//   - Creates the file (so the path is unique even if a
//     concurrent caller also calls tempPath).
//   - Closes the file handle immediately — callers want the
//     path, not an open handle, because the underlying CLI
//     (screencapture / screencapture -V / imagesnap) opens its
//     own file descriptor when writing.
//   - Returns the os.CreateTemp error verbatim on failure.
//
// Used by [Service.Screenshot], [Service.Record], and
// [Service.WebcamPhoto] in this package. The temp file is the
// caller's responsibility to delete after upload.
func tempPath(pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_ = f.Close()
	return path, nil
}
