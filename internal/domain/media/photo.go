package media

import (
	"context"
	"os"
)

// Photo captures a single-frame webcam JPEG via the optional
// `imagesnap` brew formula. The returned path is OWNED BY THE
// CALLER (same ownership rule as [Service.Screenshot]).
//
// Behavior:
//  1. Allocates a temp file via [tempPath] (under os.TempDir,
//     pattern "macontrol-photo-*.jpg"). Returns the tempPath
//     error on failure.
//  2. Runs `imagesnap -q -w 1 <path>`. The `-q` flag silences
//     the imagesnap "snap!" beep; `-w 1` warms the sensor for
//     one second before capture so exposure has time to
//     auto-adjust.
//  3. On subprocess failure, deletes the (likely empty) temp
//     file via os.Remove and returns ("", err).
//
// Permission: requires Camera TCC permission. Without it,
// imagesnap fails fast with "AVCaptureDeviceTypeBuiltInWideAngleCamera
// not authorized" — caller sees the wrapped error.
//
// Brew dependency: imagesnap is not built into macOS. If it's
// missing from $PATH, the runner returns "exec: 'imagesnap': not
// found" and the caller should suggest `brew install imagesnap`.
//
// Returns the path on success; ("", err) on any failure (with
// the temp file already cleaned up).
func (s *Service) Photo(ctx context.Context) (string, error) {
	path, err := tempPath("macontrol-photo-*.jpg")
	if err != nil {
		return "", err
	}
	// -q silences the beep; -w 1 warms the sensor for a second for a better
	// exposure.
	if _, err := s.r.Exec(ctx, "imagesnap", "-q", "-w", "1", path); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}
