// Package system exposes read-only macOS inspection — OS version, hardware
// model, memory, CPU, running processes.
package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/amiwrpremium/macontrol/internal/runner"
)

// Info is a coarse hardware + OS summary.
type Info struct {
	ProductName    string // e.g. "macOS"
	ProductVersion string // e.g. "15.3.1"
	BuildVersion   string // e.g. "24D70"
	Hostname       string
	Model          string // e.g. "MacBookPro18,3"
	ChipName       string // e.g. "Apple M3 Pro"
	CPUCores       string // e.g. "11 (6 performance and 5 efficiency)"
	TotalRAMBytes  uint64
	Uptime         string // raw `uptime` line
}

// Service bundles all read-only system helpers.
type Service struct{ r runner.Runner }

// New returns a Service.
func New(r runner.Runner) *Service { return &Service{r: r} }

// Info reads an aggregate info snapshot.
func (s *Service) Info(ctx context.Context) (Info, error) {
	i := Info{}

	if out, err := s.r.Exec(ctx, "sw_vers"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			k, v, ok := splitKV(line)
			if !ok {
				continue
			}
			switch k {
			case "ProductName":
				i.ProductName = v
			case "ProductVersion":
				i.ProductVersion = v
			case "BuildVersion":
				i.BuildVersion = v
			}
		}
	}
	if out, err := s.r.Exec(ctx, "hostname"); err == nil {
		i.Hostname = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.model"); err == nil {
		i.Model = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
		i.ChipName = strings.TrimSpace(string(out))
	}
	if out, err := s.r.Exec(ctx, "sysctl", "-n", "hw.memsize"); err == nil {
		_, _ = fmt.Sscan(strings.TrimSpace(string(out)), &i.TotalRAMBytes)
	}
	if out, err := s.r.Exec(ctx, "uptime"); err == nil {
		i.Uptime = strings.TrimSpace(string(out))
	}
	// Best-effort CPU core breakdown from system_profiler.
	if out, err := s.r.Exec(ctx, "system_profiler", "SPHardwareDataType"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Total Number of Cores:") {
				i.CPUCores = strings.TrimSpace(strings.TrimPrefix(line, "Total Number of Cores:"))
			}
		}
	}
	return i, nil
}

func splitKV(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}
