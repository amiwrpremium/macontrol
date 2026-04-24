package system

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Memory is a parsed memory snapshot composed by [Service.Memory].
// The dashboard renders the byte-count breakdown (Used / Wired /
// Compressed / Unused), the free-percentage, swap stats, and the
// top-3 RAM hogs as separate sections.
//
// Lifecycle:
//   - Constructed by Service.Memory each time the System → Memory
//     dashboard is opened or refreshed. Never cached.
//
// Field roles:
//   - UsedBytes / WiredBytes / CompressedBytes / UnusedBytes come
//     from the "PhysMem:" header line of `top -l 1`.
//   - FreePercent comes from `memory_pressure`'s "System-wide
//     memory free percentage:" line — stable across macOS
//     versions, unlike the noisier header lines.
//   - SwapUsedBytes / SwapTotalBytes come from `sysctl vm.swapusage`.
//   - TopByMem is the top-3 processes by %MEM from
//     [Service.TopByMem]. nil on failure.
//   - Raw is the verbatim "PhysMem:" line for fallback rendering.
//   - FreePercent is sentinel-valued at -1 (not 0) when unknown
//     so the renderer can distinguish "memory_pressure failed"
//     from "0% free".
type Memory struct {
	// UsedBytes is the "used" byte count from `top`'s PhysMem
	// line.
	UsedBytes uint64

	// WiredBytes is the kernel-wired byte count parsed from the
	// PhysMem line's parenthetical breakdown. Zero when not
	// reported.
	WiredBytes uint64

	// CompressedBytes is the compressor byte count from the
	// same parenthetical. Zero on macOS versions or loads that
	// don't emit a compressor breakdown.
	CompressedBytes uint64

	// UnusedBytes is the "unused" byte count from the PhysMem
	// line.
	UnusedBytes uint64

	// FreePercent is the system-wide free-memory percentage from
	// `memory_pressure`. Sentinel -1 means unknown; 0..100
	// otherwise.
	FreePercent int

	// SwapUsedBytes is the active swap size from
	// `sysctl vm.swapusage`.
	SwapUsedBytes uint64

	// SwapTotalBytes is the total swap size from
	// `sysctl vm.swapusage`.
	SwapTotalBytes uint64

	// TopByMem is up to N processes sorted by %MEM as returned
	// by [Service.TopByMem]. nil when ps failed.
	TopByMem []Process

	// Raw is the original "PhysMem:" line, preserved for
	// fallback display when parsing missed fields.
	Raw string
}

// Memory reads the memory snapshot by composing several macOS
// CLIs.
//
// Behavior:
//  1. Reads `top -l 1 -s 0` and walks for the "PhysMem:" line;
//     when found, stamps Raw and parses Used/Wired/Compressed/
//     Unused via [ParsePhysMem].
//  2. Reads `memory_pressure` and parses [ParseFreePercent].
//  3. Reads `sysctl vm.swapusage` and parses [ParseSwap].
//  4. Calls [Service.TopByMem] for the top-3 processes by %MEM.
//
// Tracks an internal `any` flag and returns a non-nil error only
// when EVERY source failed — the dashboard prefers partial info
// to a hard error.
//
// Returns the populated [Memory] (with FreePercent = -1 when not
// read) and the "could not read any memory data" error in the
// total-failure case.
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

// Compiled regexes used by the memory parsers in this file.
//
// Each regex captures the smallest stable substring needed for
// its parser. Empirically `top` formatting changes minor details
// release-to-release (spacing, comma placement) so these are
// intentionally lenient with whitespace and noisy separators.
var (
	// physMemRe captures the three top-level byte counts from
	// the `top` PhysMem line: used, the parenthetical breakdown
	// (wired + compressor inside), and unused.
	physMemRe = regexp.MustCompile(`PhysMem:\s+(\S+)\s+used\s+\(([^)]*)\),\s+(\S+)\s+unused`)

	// wiredRe matches the "<size> wired" token inside the
	// PhysMem parenthetical.
	wiredRe = regexp.MustCompile(`(\S+)\s+wired`)

	// comprRe matches the "<size> compressor" token inside the
	// PhysMem parenthetical. Optional — older macOS or lighter
	// loads omit the compressor segment.
	comprRe = regexp.MustCompile(`(\S+)\s+compressor`)

	// freePctRe matches the stable "System-wide memory free
	// percentage: N%" line from `memory_pressure`. Used in
	// preference to the noisier human-readable header lines.
	freePctRe = regexp.MustCompile(`(?i)System-wide memory free percentage:\s+(\d+)\s*%`)

	// swapRe captures total + used from
	// `sysctl vm.swapusage: total = … used = …`. The free /
	// encrypted suffixes are intentionally ignored.
	swapRe = regexp.MustCompile(`vm\.swapusage:\s+total\s*=\s*(\S+)\s+used\s*=\s*(\S+)`)
)

// ParsePhysMem extracts the four byte counts from `top`'s
// PhysMem line, e.g.
// "PhysMem: 23G used (3401M wired, 8367M compressor), 550M unused.".
//
// Behavior:
//   - On regex match, parses used + unused via [parseSizeSuffix]
//     directly from the captured groups.
//   - Then runs sub-regexes against the parenthetical to extract
//     wired + compressor. Either is optional; missing tokens
//     leave their field at zero.
//   - On no overall match, returns (0, 0, 0, 0).
//
// Exported for tests and any future caller with a captured top
// line in hand.
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

// ParseFreePercent extracts the system-wide free-memory
// percentage from `memory_pressure` output.
//
// Behavior:
//   - Matches the "System-wide memory free percentage: N%" line
//     via [freePctRe]. This line is stable across macOS releases,
//     unlike the noisier "The system has …" header lines.
//   - Returns (0, false) on no match or on Atoi failure of the
//     captured digits.
//   - Returns (N, true) on success.
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

// ParseSwap extracts swap usage from `sysctl vm.swapusage` output,
// e.g.
// "vm.swapusage: total = 2048.00M  used = 1234.56M  free = 813.44M  (encrypted)".
//
// Behavior:
//   - On regex match, parses total + used via [parseSizeSuffix].
//     Returns (0, 0) on no match.
//   - The "free" and "(encrypted)" trailing tokens are ignored —
//     the renderer derives free as total - used.
func ParseSwap(out string) (used, total uint64) {
	m := swapRe.FindStringSubmatch(out)
	if m == nil {
		return
	}
	total = parseSizeSuffix(m[1])
	used = parseSizeSuffix(m[2])
	return
}

// parseSizeSuffix converts macOS-style human-readable size
// strings ("23G", "3401M", "1234.56M", "256K", "2048.00M") to
// raw byte counts.
//
// Behavior:
//   - Trims surrounding whitespace.
//   - Inspects the last byte for a unit suffix (case-insensitive
//     K / M / G / T) and pops it off the string. Missing suffix
//     means raw bytes.
//   - Multiplies via 1024-based units (1K = 1024 bytes,
//     1M = 1024² bytes, etc.) — matching the binary base macOS
//     uses for these tools, NOT the SI base.
//   - Returns 0 on empty input or strconv.ParseFloat failure.
//   - Lossy for very large values: float64 has ~15-16 digits of
//     precision, so the conversion saturates around 8 PiB.
//     Acceptable for a Mac dashboard.
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
