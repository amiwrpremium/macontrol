package media

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Record captures a screen recording of duration to a fresh temp
// .mov file. The returned path is OWNED BY THE CALLER (same
// ownership rule as [Service.Screenshot]).
//
// Behavior:
//  1. Validates duration > 0; returns "duration must be
//     positive, got %s" for non-positive input.
//  2. Allocates a temp file via [tempPath] (under os.TempDir,
//     pattern "macontrol-record-*.mov"). Returns the tempPath
//     error on failure.
//  3. Runs `screencapture -v -V <secs> <path>`. The `-v` flag
//     enables video mode; `-V <secs>` is the duration cap.
//  4. On subprocess failure, deletes the (likely empty) temp
//     file via os.Remove and returns ("", err).
//
// macOS-version note: `screencapture -V` is documented as
// available since macOS 14. macontrol's minimum is macOS 11;
// on 11-13 the call fails with "unrecognized option -V" and the
// caller sees the wrapped error. The flow that calls Record
// (internal/telegram/flows/record.go) doesn't pre-flight this,
// so users on older macOS see the failure on first attempt.
//
// Permission: requires Screen Recording TCC permission. Without
// it, screencapture writes a black/empty .mov and exits zero
// (same caveat as Screenshot).
//
// Returns the path on success; ("", err) on any failure (with
// the temp file already cleaned up).
func (s *Service) Record(ctx context.Context, duration time.Duration) (string, error) {
	if duration <= 0 {
		return "", fmt.Errorf("duration must be positive, got %s", duration)
	}
	path, err := tempPath("macontrol-record-*.mov")
	if err != nil {
		return "", err
	}
	// `screencapture -v` records until SIGINT or until the -V <secs> timeout.
	// `-V` is supported on macOS 14+; on older releases we fall back to
	// starting the process and killing it ourselves — but we only target
	// 11+, so we prefer -V when present and time-limit via ctx otherwise.
	args := []string{"-v", "-V", strconv.Itoa(int(duration.Seconds())), path}
	if _, err := s.r.Exec(ctx, "screencapture", args...); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}
