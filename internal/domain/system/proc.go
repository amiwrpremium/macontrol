package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Process is a single row from `ps`.
type Process struct {
	PID     int
	CPU     float64 // %
	Mem     float64 // %
	Command string
}

// TopN returns the top N processes by CPU%.
func (s *Service) TopN(ctx context.Context, n int) ([]Process, error) {
	return s.topNBySort(ctx, n, "-r")
}

// TopByMem returns the top N processes by RAM% (defaults to 3 when
// n <= 0). Uses `ps -m` to sort by resident memory.
func (s *Service) TopByMem(ctx context.Context, n int) ([]Process, error) {
	if n <= 0 {
		n = 3
	}
	return s.topNBySort(ctx, n, "-m")
}

// topNBySort runs `ps -Ao pid,pcpu,pmem,comm <sortFlag>` and parses
// the first n rows. sortFlag is `-r` (CPU) or `-m` (memory).
func (s *Service) topNBySort(ctx context.Context, n int, sortFlag string) ([]Process, error) {
	if n <= 0 {
		n = 10
	}
	out, err := s.r.Exec(ctx, "ps", "-Ao", "pid,pcpu,pmem,comm", sortFlag)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) <= 1 {
		return nil, nil
	}
	// Drop header.
	lines = lines[1:]
	result := make([]Process, 0, n)
	for _, raw := range lines {
		if len(result) >= n {
			break
		}
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		mem, _ := strconv.ParseFloat(fields[2], 64)
		cmd := strings.Join(fields[3:], " ")
		result = append(result, Process{PID: pid, CPU: cpu, Mem: mem, Command: cmd})
	}
	return result, nil
}

// Kill sends SIGTERM to a PID.
func (s *Service) Kill(ctx context.Context, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("pid must be positive, got %d", pid)
	}
	_, err := s.r.Exec(ctx, "kill", strconv.Itoa(pid))
	return err
}

// KillByName sends SIGTERM to every process matching name.
func (s *Service) KillByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	_, err := s.r.Exec(ctx, "killall", name)
	return err
}
