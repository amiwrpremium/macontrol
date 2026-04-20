package system

import (
	"context"
	"fmt"
	"strings"
)

// Memory is a coarse memory snapshot.
type Memory struct {
	PressureLevel string // from `memory_pressure` header line
	// Raw output of `memory_pressure` and `vm_stat` — we render these as
	// monospace blocks rather than trying to pull every field.
	MemoryPressureRaw string
	VMStatRaw         string
	PhysMemSummary    string // from `top -l 1 -s 0`
}

// Memory reads an aggregate memory snapshot.
func (s *Service) Memory(ctx context.Context) (Memory, error) {
	m := Memory{}
	if out, err := s.r.Exec(ctx, "memory_pressure"); err == nil {
		m.MemoryPressureRaw = string(out)
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "The system has") {
				m.PressureLevel = line
				break
			}
		}
	}
	if out, err := s.r.Exec(ctx, "vm_stat"); err == nil {
		m.VMStatRaw = string(out)
	}
	if out, err := s.r.Exec(ctx, "top", "-l", "1", "-s", "0"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "PhysMem:") {
				m.PhysMemSummary = line
				break
			}
		}
	}
	if m.PhysMemSummary == "" && m.MemoryPressureRaw == "" && m.VMStatRaw == "" {
		return m, fmt.Errorf("could not read any memory data")
	}
	return m, nil
}
