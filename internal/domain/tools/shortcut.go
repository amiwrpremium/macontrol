package tools

import (
	"context"
	"strings"
)

// ShortcutsList returns user Shortcuts names (requires `shortcuts` CLI,
// macOS 13+).
func (s *Service) ShortcutsList(ctx context.Context) ([]string, error) {
	out, err := s.r.Exec(ctx, "shortcuts", "list")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// ShortcutRun executes a user Shortcut by name.
func (s *Service) ShortcutRun(ctx context.Context, name string) error {
	_, err := s.r.Exec(ctx, "shortcuts", "run", name)
	return err
}
