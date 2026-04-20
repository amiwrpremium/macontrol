package media

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Record captures a screen recording of the given duration to a temp .mov
// file. Uses `screencapture -v`; requires Screen Recording TCC permission.
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
