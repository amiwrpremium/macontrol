package system

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Memory is a parsed memory snapshot. All numeric fields are zero
// when their source couldn't be read or parsed; FreePercent is -1
// when unknown. Raw preserves the `top` PhysMem line for fallback
// rendering.
type Memory struct {
	// UsedBytes is the "used" byte count from `top`'s PhysMem line.
	UsedBytes uint64
	// WiredBytes is the kernel-wired byte count; 0 when not reported.
	WiredBytes uint64
	// CompressedBytes is the compressor byte count; 0 on macOS
	// versions or loads that don't emit a compressor breakdown.
	CompressedBytes uint64
	// UnusedBytes is the "unused" byte count from the PhysMem line.
	UnusedBytes uint64
	// FreePercent is the system-wide free-memory percentage from
	// `memory_pressure`; -1 when unknown, 0..100 otherwise.
	FreePercent int
	// SwapUsedBytes is the active swap size from `sysctl vm.swapusage`.
	SwapUsedBytes uint64
	// SwapTotalBytes is the total swap size from `sysctl vm.swapusage`.
	SwapTotalBytes uint64
	// TopByMem is up to N processes sorted by RAM%; nil when `ps`
	// failed.
	TopByMem []Process
	// Raw is the raw `top` PhysMem line, kept for fallback display
	// when parsing fails.
	Raw string
}

// Memory reads a parsed memory snapshot. Best-effort: any source
// that fails leaves its fields zero / nil. Returns an error only if
// every source failed.
func (s *Service) Memory(ctx context.Context) (Memory, error) {
	m := Memory{FreePercent: -1}
	any := false

	if out, err := s.r.Exec(ctx, "top", "-l", "1", "-s", "0"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "PhysMem:") {
				continue
			}
			m.Raw = line
			m.UsedBytes, m.WiredBytes, m.CompressedBytes, m.UnusedBytes = ParsePhysMem(line)
			any = true
			break
		}
	}
	if out, err := s.r.Exec(ctx, "memory_pressure"); err == nil {
		if pct, ok := ParseFreePercent(string(out)); ok {
			m.FreePercent = pct
			any = true
		}
	}
	if out, err := s.r.Exec(ctx, "sysctl", "vm.swapusage"); err == nil {
		m.SwapUsedBytes, m.SwapTotalBytes = ParseSwap(string(out))
		if m.SwapTotalBytes > 0 {
			any = true
		}
	}
	if procs, err := s.TopByMem(ctx, 3); err == nil && len(procs) > 0 {
		m.TopByMem = procs
		any = true
	}

	if !any {
		return m, fmt.Errorf("could not read any memory data")
	}
	return m, nil
}

var (
	physMemRe = regexp.MustCompile(`PhysMem:\s+(\S+)\s+used\s+\(([^)]*)\),\s+(\S+)\s+unused`)
	wiredRe   = regexp.MustCompile(`(\S+)\s+wired`)
	comprRe   = regexp.MustCompile(`(\S+)\s+compressor`)
	freePctRe = regexp.MustCompile(`(?i)System-wide memory free percentage:\s+(\d+)\s*%`)
	swapRe    = regexp.MustCompile(`vm\.swapusage:\s+total\s*=\s*(\S+)\s+used\s*=\s*(\S+)`)
)

// ParsePhysMem extracts byte counts from `top`'s PhysMem line, e.g.
// "PhysMem: 23G used (3401M wired, 8367M compressor), 550M unused."
// Compressor segment is optional (older macOS or lighter loads).
func ParsePhysMem(line string) (used, wired, compressed, unused uint64) {
	m := physMemRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	used = parseSizeSuffix(m[1])
	unused = parseSizeSuffix(m[3])
	if w := wiredRe.FindStringSubmatch(m[2]); w != nil {
		wired = parseSizeSuffix(w[1])
	}
	if c := comprRe.FindStringSubmatch(m[2]); c != nil {
		compressed = parseSizeSuffix(c[1])
	}
	return
}

// ParseFreePercent finds the "System-wide memory free percentage: N%"
// line in `memory_pressure` output. This line is stable across macOS
// versions, unlike the noisy "The system has …" header lines.
func ParseFreePercent(out string) (int, bool) {
	m := freePctRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// ParseSwap extracts swap usage from `sysctl vm.swapusage`, e.g.
// "vm.swapusage: total = 2048.00M  used = 1234.56M  free = 813.44M  (encrypted)".
func ParseSwap(out string) (used, total uint64) {
	m := swapRe.FindStringSubmatch(out)
	if m == nil {
		return
	}
	total = parseSizeSuffix(m[1])
	used = parseSizeSuffix(m[2])
	return
}

// parseSizeSuffix converts strings like "23G", "3401M", "1234.56M",
// "256K", "2048.00M" to bytes. Returns 0 on parse failure.
func parseSizeSuffix(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := uint64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		mult = 1 << 10
		s = s[:len(s)-1]
	case 'M', 'm':
		mult = 1 << 20
		s = s[:len(s)-1]
	case 'G', 'g':
		mult = 1 << 30
		s = s[:len(s)-1]
	case 'T', 't':
		mult = 1 << 40
		s = s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return uint64(v * float64(mult))
}
