// Package media handles screenshots, screen recording, and webcam snapshots.
// Callers are responsible for cleaning up the returned temp files.
package media

import (
	"context"
	"os"
	"strconv"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Service produces still images and recordings of the Mac.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// ScreenshotOpts tunes `screencapture`.
type ScreenshotOpts struct {
	Display int  // 0 = all, 1/2/… = specific display
	Silent  bool // suppress shutter sound
	Delay   int  // seconds before capture (0 = immediate)
}

// Screenshot captures the screen to a fresh temp file. The returned path is
// owned by the caller.
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

func tempPath(pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_ = f.Close()
	return path, nil
}
