package system_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// ---------------- info.go ----------------

func TestInfo_FullHappyPath(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("sw_vers", "ProductName: macOS\nProductVersion: 15.3.1\nBuildVersion: 24D70\n", nil).
		On("hostname", "tower.local\n", nil).
		On("sysctl -n hw.model", "MacBookPro18,3\n", nil).
		On("sysctl -n machdep.cpu.brand_string", "Apple M3 Pro\n", nil).
		On("sysctl -n hw.memsize", "34359738368\n", nil).
		On("uptime", "21:44  up 3 days,  6:27, 1 user, load averages: 4.97 4.57 4.19\n", nil).
		On("system_profiler SPHardwareDataType",
			"  Hardware Overview:\n  Total Number of Cores: 11 (6 performance and 5 efficiency)\n", nil)

	info, err := system.New(f).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.ProductVersion != "15.3.1" || info.BuildVersion != "24D70" {
		t.Errorf("sw_vers fields: %+v", info)
	}
	if info.Hostname != "tower.local" {
		t.Errorf("hostname = %q", info.Hostname)
	}
	if info.Model != "MacBookPro18,3" {
		t.Errorf("model = %q", info.Model)
	}
	if info.ChipName != "Apple M3 Pro" {
		t.Errorf("chip = %q", info.ChipName)
	}
	if info.TotalRAMBytes != 34359738368 {
		t.Errorf("ram = %d", info.TotalRAMBytes)
	}
	if info.Uptime.Duration != "3 days,  6h 27m" {
		t.Errorf("uptime duration = %q", info.Uptime.Duration)
	}
	if info.Uptime.Users != 1 {
		t.Errorf("uptime users = %d", info.Uptime.Users)
	}
	if info.Uptime.Load1 != 4.97 || info.Uptime.Load5 != 4.57 || info.Uptime.Load15 != 4.19 {
		t.Errorf("load avg = %v / %v / %v", info.Uptime.Load1, info.Uptime.Load5, info.Uptime.Load15)
	}
	if !strings.Contains(info.CPUCores, "11") {
		t.Errorf("cores = %q", info.CPUCores)
	}
}

func TestParseUptime_Variants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		raw   string
		want  system.Uptime
		check func(*testing.T, system.Uptime)
	}{
		{
			name: "live macos26 line",
			raw:  "21:44  up 3 days,  6:27, 1 user, load averages: 4.97 4.57 4.19",
			check: func(t *testing.T, u system.Uptime) {
				if u.Duration != "3 days,  6h 27m" {
					t.Errorf("duration = %q", u.Duration)
				}
				if u.Users != 1 || u.Load1 != 4.97 || u.Load15 != 4.19 {
					t.Errorf("got %+v", u)
				}
			},
		},
		{
			name: "short uptime, singular load average",
			raw:  "10:00  up 47 mins, 1 user, load average: 0.5 0.3 0.2",
			check: func(t *testing.T, u system.Uptime) {
				if u.Duration != "47 mins" {
					t.Errorf("duration = %q", u.Duration)
				}
				if u.Load1 != 0.5 {
					t.Errorf("load1 = %v", u.Load1)
				}
			},
		},
		{
			name: "sub-day HH:MM, plural users",
			raw:  "10:00  up 18:23, 2 users, load averages: 1 2 3",
			check: func(t *testing.T, u system.Uptime) {
				if u.Duration != "18h 23m" {
					t.Errorf("duration = %q", u.Duration)
				}
				if u.Users != 2 {
					t.Errorf("users = %d", u.Users)
				}
			},
		},
		{
			name: "garbage line preserves raw, leaves rest zero",
			raw:  "this is not uptime output",
			check: func(t *testing.T, u system.Uptime) {
				if u.Raw != "this is not uptime output" {
					t.Errorf("raw = %q", u.Raw)
				}
				if u.Duration != "" || u.Users != 0 || u.Load1 != 0 {
					t.Errorf("expected zero parsed fields, got %+v", u)
				}
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFake().
				On("sw_vers", "", errors.New("skip")).
				On("hostname", "", errors.New("skip")).
				On("sysctl -n hw.model", "", errors.New("skip")).
				On("sysctl -n machdep.cpu.brand_string", "", errors.New("skip")).
				On("sysctl -n hw.memsize", "", errors.New("skip")).
				On("uptime", c.raw+"\n", nil).
				On("system_profiler SPHardwareDataType", "", errors.New("skip"))
			info, _ := system.New(f).Info(context.Background())
			c.check(t, info.Uptime)
		})
	}
}

func TestFirstInt(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want int
		ok   bool
	}{
		"12 (8 performance and 4 efficiency)": {12, true},
		"11":                                  {11, true},
		"":                                    {0, false},
		"no digits here":                      {0, false},
		"  42  trailing junk":                 {42, true},
	}
	for in, want := range cases {
		got, ok := system.FirstInt(in)
		if got != want.want || ok != want.ok {
			t.Errorf("FirstInt(%q) = %d, %v; want %d, %v", in, got, ok, want.want, want.ok)
		}
	}
}

func TestInfo_PartialFailuresDegradeGracefully(t *testing.T) {
	t.Parallel()
	// Only hostname succeeds; everything else fails. Info should still
	// return nil error (best-effort aggregate) with only hostname populated.
	f := runner.NewFake().
		On("sw_vers", "", errors.New("x")).
		On("hostname", "h.local\n", nil).
		On("sysctl -n hw.model", "", errors.New("x")).
		On("sysctl -n machdep.cpu.brand_string", "", errors.New("x")).
		On("sysctl -n hw.memsize", "", errors.New("x")).
		On("uptime", "", errors.New("x")).
		On("system_profiler SPHardwareDataType", "", errors.New("x"))
	info, err := system.New(f).Info(context.Background())
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got %v", err)
	}
	if info.Hostname != "h.local" {
		t.Errorf("hostname = %q", info.Hostname)
	}
	if info.ProductVersion != "" || info.Model != "" {
		t.Errorf("expected zero values for failed reads: %+v", info)
	}
}

// ---------------- temp.go ----------------

func TestThermal_PressureLevels(t *testing.T) {
	t.Parallel()
	levels := []string{"Nominal", "Moderate", "Heavy", "Trapping", "Sleeping"}
	for _, lvl := range levels {
		lvl := lvl
		t.Run(lvl, func(t *testing.T) {
			t.Parallel()
			out := "prelude\nCurrent pressure level: " + lvl + "\nother stuff\n"
			f := runner.NewFake().
				On("powermetrics -n 1 -i 1000 --samplers thermal", out, nil).
				On("smctemp -c", "", errors.New("missing")).
				On("smctemp -g", "", errors.New("missing"))
			th, err := system.New(f).Thermal(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if string(th.Pressure) != lvl {
				t.Fatalf("pressure = %q", th.Pressure)
			}
		})
	}
}

func TestThermal_SmctempAvailable(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("powermetrics -n 1 -i 1000 --samplers thermal",
			"Current pressure level: Nominal\n", nil).
		On("smctemp -c", "52.7\n", nil).
		On("smctemp -g", "47.1\n", nil)
	th, err := system.New(f).Thermal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !th.SmctempAvail {
		t.Error("expected SmctempAvail=true")
	}
	if th.CPUTempC < 52 || th.CPUTempC > 53 {
		t.Errorf("cpu = %f", th.CPUTempC)
	}
	if th.GPUTempC < 47 || th.GPUTempC > 48 {
		t.Errorf("gpu = %f", th.GPUTempC)
	}
}

func TestThermal_SmctempMissing(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("powermetrics -n 1 -i 1000 --samplers thermal",
			"Current pressure level: Nominal\n", nil).
		On("smctemp -c", "", errors.New("not found")).
		On("smctemp -g", "", errors.New("not found"))
	th, err := system.New(f).Thermal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if th.SmctempAvail {
		t.Error("expected SmctempAvail=false")
	}
	if th.CPUTempC != 0 || th.GPUTempC != 0 {
		t.Errorf("expected zero temps; got %+v", th)
	}
}

func TestThermal_SmctempEmptyOutput(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("powermetrics -n 1 -i 1000 --samplers thermal",
			"Current pressure level: Nominal\n", nil).
		On("smctemp -c", "\n", nil).
		On("smctemp -g", "", errors.New("missing"))
	th, err := system.New(f).Thermal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Empty smctemp is handled as "not available" — no error, CPUTempC stays 0.
	if th.CPUTempC != 0 {
		t.Errorf("cpu = %f", th.CPUTempC)
	}
}

func TestThermal_PowermetricsUnavailable(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("powermetrics -n 1 -i 1000 --samplers thermal", "", errors.New("no sudo")).
		On("smctemp -c", "", errors.New("x")).
		On("smctemp -g", "", errors.New("x"))
	th, err := system.New(f).Thermal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if th.Pressure != "unknown" {
		t.Errorf("pressure = %q", th.Pressure)
	}
}

// ---------------- mem.go ----------------

func TestMemory_AllParts(t *testing.T) {
	t.Parallel()
	pressure := "The system has 36720 pages free out of 8388608.\nSystem-wide memory free percentage: 70%\n"
	vmstat := "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages free: 36720\n"
	top := "Processes: 500 total\nPhysMem: 18G used (2G wired), 6G unused.\n"
	f := runner.NewFake().
		On("memory_pressure", pressure, nil).
		On("vm_stat", vmstat, nil).
		On("top -l 1 -s 0", top, nil)
	m, err := system.New(f).Memory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.PressureLevel, "system has") {
		t.Errorf("pressureLevel = %q", m.PressureLevel)
	}
	if !strings.Contains(m.VMStatRaw, "Pages free") {
		t.Error("vmstat missing")
	}
	if !strings.Contains(m.PhysMemSummary, "PhysMem") {
		t.Errorf("physmem = %q", m.PhysMemSummary)
	}
}

func TestMemory_AllFail(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("memory_pressure", "", errors.New("x")).
		On("vm_stat", "", errors.New("x")).
		On("top -l 1 -s 0", "", errors.New("x"))
	if _, err := system.New(f).Memory(context.Background()); err == nil {
		t.Fatal("expected error when no data at all")
	}
}

// ---------------- cpu.go ----------------

func TestCPU_Parses(t *testing.T) {
	t.Parallel()
	top := "Processes: 500 total\nCPU usage: 5.10% user, 3.20% sys, 91.70% idle\n"
	f := runner.NewFake().
		On("uptime", " 10:00 up 1 day, load average: 0.5 0.6 0.7\n", nil).
		On("top -l 1 -s 0", top, nil)
	c, err := system.New(f).CPU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.LoadAverage, "load average") {
		t.Errorf("load = %q", c.LoadAverage)
	}
	if !strings.Contains(c.TopHeader, "CPU usage") {
		t.Errorf("top = %q", c.TopHeader)
	}
}

func TestCPU_UptimeFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("uptime", "", errors.New("x")).
		On("top -l 1 -s 0", "CPU usage: 1% user\n", nil)
	c, err := system.New(f).CPU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.LoadAverage != "" {
		t.Errorf("load = %q", c.LoadAverage)
	}
}

// ---------------- proc.go ----------------

func TestTopN_DefaultTen(t *testing.T) {
	t.Parallel()
	header := "  PID  %CPU %MEM COMM\n"
	var body strings.Builder
	for i := 1; i <= 20; i++ {
		// lines like:  100 10.5  3.2 /Applications/App.app
		body.WriteString("  100  10.5  3.2 /Applications/App.app\n")
		_ = i
	}
	f := runner.NewFake().On("ps -Ao pid,pcpu,pmem,comm -r", header+body.String(), nil)
	procs, err := system.New(f).TopN(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 10 {
		t.Fatalf("expected default 10, got %d", len(procs))
	}
}

func TestTopN_RespectsN(t *testing.T) {
	t.Parallel()
	header := "  PID  %CPU %MEM COMM\n"
	var body strings.Builder
	for i := 1; i <= 5; i++ {
		body.WriteString("  100  10.5  3.2 /App\n")
	}
	f := runner.NewFake().On("ps -Ao pid,pcpu,pmem,comm -r", header+body.String(), nil)
	procs, _ := system.New(f).TopN(context.Background(), 3)
	if len(procs) != 3 {
		t.Fatalf("got %d want 3", len(procs))
	}
}

func TestTopN_EmptyOutput(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("ps -Ao pid,pcpu,pmem,comm -r", "", nil)
	procs, err := system.New(f).TopN(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if procs != nil {
		t.Fatalf("expected nil, got %+v", procs)
	}
}

func TestTopN_MalformedRowsSkipped(t *testing.T) {
	t.Parallel()
	data := "  PID  %CPU %MEM COMM\n" +
		"  100 10.5  3.2 /App\n" +
		"shortrow\n" +
		"  200 11.0  4.5 /Other\n"
	f := runner.NewFake().On("ps -Ao pid,pcpu,pmem,comm -r", data, nil)
	procs, err := system.New(f).TopN(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 {
		t.Fatalf("got %d want 2 (malformed skipped)", len(procs))
	}
}

func TestTopN_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("ps -Ao pid,pcpu,pmem,comm -r", "", errors.New("x"))
	if _, err := system.New(f).TopN(context.Background(), 3); err == nil {
		t.Fatal("expected error")
	}
}

func TestKill_Success(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill 123", "", nil)
	if err := system.New(f).Kill(context.Background(), 123); err != nil {
		t.Fatal(err)
	}
}

func TestKill_RejectsNonPositive(t *testing.T) {
	t.Parallel()
	svc := system.New(runner.NewFake())
	for _, pid := range []int{0, -1, -999} {
		if err := svc.Kill(context.Background(), pid); err == nil {
			t.Errorf("pid=%d should be rejected", pid)
		}
	}
}

func TestKill_Propagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("kill 1", "", errors.New("No such process"))
	if err := system.New(f).Kill(context.Background(), 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestKillByName(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("killall Safari", "", nil)
	if err := system.New(f).KillByName(context.Background(), "Safari"); err != nil {
		t.Fatal(err)
	}
}

func TestKillByName_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if err := system.New(runner.NewFake()).KillByName(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestKillByName_Propagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("killall nope", "", errors.New("no processes"))
	if err := system.New(f).KillByName(context.Background(), "nope"); err == nil {
		t.Fatal("expected error")
	}
}
