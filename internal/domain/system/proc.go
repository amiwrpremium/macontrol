package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Process is one row from `ps -Ao pid,pcpu,pmem,comm`. Used by
// the System dashboard for the top-CPU and top-memory drill-down
// lists, and by the per-process kill panel.
//
// Lifecycle:
//   - Constructed by [Service.TopN] / [Service.TopByMem] each
//     time the dashboard refreshes. Never cached.
//   - The PID is used as the lookup key by [Service.Kill] and
//     [Service.KillForce].
//
// Field roles:
//   - PID is the kernel process identifier as reported by `ps`.
//   - CPU and Mem are the percent-of-system values from `ps`'s
//     pcpu / pmem columns; both are floats so partial percentages
//     (0.3, 12.4) round-trip.
//   - Command is the comm column joined on spaces. macOS truncates
//     comm at 16 chars by default but `ps -A` overrides that and
//     emits the full path (e.g. "/Applications/Foo.app/Contents/MacOS/Foo").
//     The keyboard layer extracts the basename for display via
//     leafOf in keyboards/sys.go.
type Process struct {
	// PID is the kernel process identifier.
	PID int

	// CPU is the current %CPU as reported by `ps -o pcpu`.
	CPU float64

	// Mem is the current %MEM as reported by `ps -o pmem`.
	Mem float64

	// Command is the comm column from `ps`, joined on spaces.
	// Typically a full path; the keyboard layer extracts a
	// basename for display.
	Command string
}

// TopN returns the top n processes sorted by %CPU (descending).
// Delegates to [Service.topNBySort] with the `-r` flag (ps's
// "sort by CPU" alias).
//
// Pass n <= 0 to get the default of 10 entries; the helper
// caps internally.
func (s *Service) TopN(ctx context.Context, n int) ([]Process, error) {
	return s.topNBySort(ctx, n, "-r")
}

// TopByMem returns the top n processes sorted by %MEM (descending).
// Delegates to [Service.topNBySort] with the `-m` flag.
//
// Behavior:
//   - n <= 0 is treated as 3 (one fewer than TopN's default
//     because the memory dashboard renders fewer hogs by default).
func (s *Service) TopByMem(ctx context.Context, n int) ([]Process, error) {
	if n <= 0 {
		n = 3
	}
	return s.topNBySort(ctx, n, "-m")
}

// topNBySort runs `ps -Ao pid,pcpu,pmem,comm <sortFlag>` and parses
// the first n rows into [Process] entries. sortFlag is `-r` (sort
// by CPU) or `-m` (sort by memory).
//
// Behavior:
//   - n <= 0 is treated as 10.
//   - Drops the header row before parsing.
//   - Parses fields as: pid (int), pcpu (float), pmem (float),
//     and joins the remaining fields as the command (so commands
//     containing spaces survive).
//   - Atoi / ParseFloat errors are silently swallowed (each
//     remaining at zero) so a single malformed row doesn't drop
//     the entire result.
//
// Returns the parsed slice (may be empty when ps had no
// non-header rows) or the underlying ps error.
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

// Kill sends SIGTERM (the polite request to terminate) to pid.
// The target process can install a signal handler and clean up.
//
// Behavior:
//   - Validates pid > 0; returns "pid must be positive, got N"
//     for non-positive input. Defends against the obvious "kill
//     pid 0" / "kill pid -1" footguns.
//   - Shells out to `kill <pid>` (no signal flag → default
//     SIGTERM).
//   - Returns the runner error (which carries the kill stderr
//     text) on failure. Common failure: "No such process" when
//     the pid no longer exists, or "Operation not permitted"
//     when the daemon doesn't own the target.
func (s *Service) Kill(ctx context.Context, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("pid must be positive, got %d", pid)
	}
	_, err := s.r.Exec(ctx, "kill", strconv.Itoa(pid))
	return err
}

// KillForce sends SIGKILL (the uncatchable terminate) to pid.
// The process is killed immediately by the kernel with no chance
// to clean up — open files may be left in inconsistent states,
// child processes are reparented to launchd.
//
// Behavior:
//   - Same pid validation as [Service.Kill].
//   - Shells out to `kill -9 <pid>`.
//   - Same error shape as Kill.
//
// The Telegram UX gates this behind a Confirm/Cancel dialog
// because there's no recovery path for a SIGKILL'd process.
func (s *Service) KillForce(ctx context.Context, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("pid must be positive, got %d", pid)
	}
	_, err := s.r.Exec(ctx, "kill", "-9", strconv.Itoa(pid))
	return err
}

// KillByName sends SIGTERM to every process matching name via
// the `killall` CLI. Used by the typed-name kill flow when the
// target isn't in the current Top-10 list.
//
// Behavior:
//   - Rejects empty name with "name is required".
//   - Shells out to `killall <name>`. macOS killall matches by
//     comm column, NOT by full path — passing "Foo" will kill
//     every process named Foo regardless of which directory
//     they were launched from.
//   - Returns the runner error on failure. "No matching
//     processes belonging to you were found" is the typical
//     no-match error.
func (s *Service) KillByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	_, err := s.r.Exec(ctx, "killall", name)
	return err
}
