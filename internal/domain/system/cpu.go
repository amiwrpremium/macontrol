package system

import (
	"context"
	"strings"
)

// CPU is a coarse CPU snapshot.
type CPU struct {
	LoadAverage string // e.g. "load averages: 1.23 1.45 1.67"
	TopHeader   string // "CPU usage: ..." line from `top`
}

// CPU reads an aggregate CPU snapshot.
func (s *Service) CPU(ctx context.Context) (CPU, error) {
	c := CPU{}
	if out, err := s.r.Exec(ctx, "uptime"); err == nil {
		c.LoadAverage = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "top", "-l", "1", "-s", "0"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "CPU usage:") {
				c.TopHeader = line
				break
			}
		}
	}
	return c, nil
}
