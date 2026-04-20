package media

import (
	"context"
	"os"
)

// Photo snaps a webcam photo with `imagesnap` (brew: imagesnap). Requires
// Camera TCC permission. Returns the path to a jpeg.
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
