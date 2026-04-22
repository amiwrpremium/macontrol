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
	// Real macOS 26 shape: noisy "The system has …" intro then the
	// stable percentage line.
	pressure := "The system has 25769803776 (1572864 pages with a page size of 16384).\n" +
		"The system has 4938395648 (301367 pages) wired down.\n" +
		"System-wide memory free percentage: 18%\n"
	top := "Processes: 500 total\nPhysMem: 23G used (3401M wired, 8367M compressor), 550M unused.\n"
	swap := "vm.swapusage: total = 2048.00M  used = 1234.56M  free = 813.44M  (encrypted)\n"
	psOut := "  PID  %CPU %MEM COMM\n" +
		"  100  10.5 12.4 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome\n" +
		"  101   3.1  8.7 /Applications/Slack.app/Contents/MacOS/Slack\n" +
		"  102   1.0  5.1 WindowServer\n"
	f := runner.NewFake().
		On("top -l 1 -s 0", top, nil).
		On("memory_pressure", pressure, nil).
		On("sysctl vm.swapusage", swap, nil).
		On("ps -Ao pid,pcpu,pmem,comm -m", psOut, nil)
	m, err := system.New(f).Memory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m.UsedBytes != 23*(1<<30) {
		t.Errorf("used = %d bytes", m.UsedBytes)
	}
	if m.WiredBytes != 3401*(1<<20) {
		t.Errorf("wired = %d", m.WiredBytes)
	}
	if m.CompressedBytes != 8367*(1<<20) {
		t.Errorf("compressed = %d", m.CompressedBytes)
	}
	if m.UnusedBytes != 550*(1<<20) {
		t.Errorf("unused = %d", m.UnusedBytes)
	}
	if m.FreePercent != 18 {
		t.Errorf("free%% = %d (regression: parser must skip the 'system has' intro lines)", m.FreePercent)
	}
	if m.SwapTotalBytes == 0 || m.SwapUsedBytes == 0 {
		t.Errorf("swap = %d / %d", m.SwapUsedBytes, m.SwapTotalBytes)
	}
	if len(m.TopByMem) != 3 || m.TopByMem[0].Mem < 12 {
		t.Errorf("topByMem = %+v", m.TopByMem)
	}
	if !strings.Contains(m.Raw, "PhysMem:") {
		t.Errorf("raw should preserve PhysMem line; got %q", m.Raw)
	}
}

func TestMemory_AllFail(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("top -l 1 -s 0", "", errors.New("x")).
		On("memory_pressure", "", errors.New("x")).
		On("sysctl vm.swapusage", "", errors.New("x")).
		On("ps -Ao pid,pcpu,pmem,comm -m", "", errors.New("x"))
	if _, err := system.New(f).Memory(context.Background()); err == nil {
		t.Fatal("expected error when no data at all")
	}
}

func TestParsePhysMem_Variants(t *testing.T) {
	t.Parallel()
	// With compressor (modern macOS).
	used, wired, compressed, unused := system.ParsePhysMem(
		"PhysMem: 23G used (3401M wired, 8367M compressor), 550M unused.")
	if used != 23*(1<<30) || wired != 3401*(1<<20) || compressed != 8367*(1<<20) || unused != 550*(1<<20) {
		t.Errorf("with compressor: used=%d wired=%d compressed=%d unused=%d", used, wired, compressed, unused)
	}
	// Without compressor (older macOS / lighter loads).
	used, wired, compressed, unused = system.ParsePhysMem("PhysMem: 18G used (2G wired), 6G unused.")
	if used != 18*(1<<30) || wired != 2*(1<<30) || compressed != 0 || unused != 6*(1<<30) {
		t.Errorf("no compressor: used=%d wired=%d compressed=%d unused=%d", used, wired, compressed, unused)
	}
	// K-suffixed (small values, defensive).
	used, _, _, _ = system.ParsePhysMem("PhysMem: 512K used (256K wired), 128K unused.")
	if used != 512*(1<<10) {
		t.Errorf("K-suffix: used=%d", used)
	}
	// Unparseable line — all zero.
	used, wired, compressed, unused = system.ParsePhysMem("garbage line")
	if used != 0 || wired != 0 || compressed != 0 || unused != 0 {
		t.Errorf("garbage should yield zeros; got %d/%d/%d/%d", used, wired, compressed, unused)
	}
}

func TestParseFreePercent(t *testing.T) {
	t.Parallel()
	// macOS 15 fixture style.
	pct, ok := system.ParseFreePercent(
		"The system has 36720 pages free out of 8388608.\nSystem-wide memory free percentage: 70%\n")
	if !ok || pct != 70 {
		t.Errorf("macos15: got %d, ok=%v", pct, ok)
	}
	// macOS 26 fixture — the "system has" intro line is now noisy and
	// must NOT be picked. Only the percentage line counts.
	pct, ok = system.ParseFreePercent(
		"The system has 25769803776 (1572864 pages with a page size of 16384).\n" +
			"System-wide memory free percentage: 18%\n")
	if !ok || pct != 18 {
		t.Errorf("macos26: got %d, ok=%v", pct, ok)
	}
	// Missing percentage line.
	if _, ok := system.ParseFreePercent("nothing here\n"); ok {
		t.Error("expected ok=false when no percentage line")
	}
}

func TestParseSwap(t *testing.T) {
	t.Parallel()
	used, total := system.ParseSwap("vm.swapusage: total = 2048.00M  used = 1234.56M  free = 813.44M  (encrypted)")
	if total != uint64(2048*(1<<20)) {
		t.Errorf("total = %d", total)
	}
	mib := float64(uint64(1) << 20)
	wantUsed := uint64(1234.56 * mib)
	if used != wantUsed {
		t.Errorf("used = %d, want %d", used, wantUsed)
	}
	// Zero swap — both fields parse to 0 cleanly.
	used, total = system.ParseSwap("vm.swapusage: total = 0.00M  used = 0.00M  free = 0.00M")
	if used != 0 || total != 0 {
		t.Errorf("zero swap: used=%d total=%d", used, total)
	}
	// Garbage.
	used, total = system.ParseSwap("not swap output")
	if used != 0 || total != 0 {
		t.Errorf("garbage: used=%d total=%d", used, total)
	}
}

// ---------------- cpu.go ----------------

func TestCPU_Parses(t *testing.T) {
	t.Parallel()
	top := "Processes: 500 total\nCPU usage: 20.85% user, 16.25% sys, 62.88% idle\n"
	psOut := "  PID  %CPU %MEM COMM\n" +
		"  100 12.4  1.0 /Applications/Chrome\n" +
		"  101  8.7  0.5 some-process\n" +
		"  102  5.1  0.2 WindowServer\n"
	f := runner.NewFake().
		On("uptime", "21:46  up 3 days,  6:29, 1 user, load averages: 5.41 4.92 4.39\n", nil).
		On("top -l 1 -s 0", top, nil).
		On("ps -Ao pid,pcpu,pmem,comm -r", psOut, nil)
	c, err := system.New(f).CPU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.UserPct != 20.85 || c.SysPct != 16.25 || c.IdlePct != 62.88 {
		t.Errorf("usage: user=%v sys=%v idle=%v", c.UserPct, c.SysPct, c.IdlePct)
	}
	if c.Load1 != 5.41 || c.Load5 != 4.92 || c.Load15 != 4.39 {
		t.Errorf("load = %v / %v / %v", c.Load1, c.Load5, c.Load15)
	}
	if len(c.TopByCPU) != 3 || c.TopByCPU[0].CPU < 12 {
		t.Errorf("topByCPU = %+v", c.TopByCPU)
	}
	if !strings.Contains(c.Raw, "CPU usage:") {
		t.Errorf("raw should preserve top line; got %q", c.Raw)
	}
}

func TestCPU_UptimeFails(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().
		On("uptime", "", errors.New("x")).
		On("top -l 1 -s 0", "CPU usage: 1.0% user, 1.0% sys, 98.0% idle\n", nil).
		On("ps -Ao pid,pcpu,pmem,comm -r", "", errors.New("x"))
	c, err := system.New(f).CPU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.Load1 != 0 || c.Load5 != 0 || c.Load15 != 0 {
		t.Errorf("load should be zero on uptime failure; got %v/%v/%v", c.Load1, c.Load5, c.Load15)
	}
	if c.UserPct != 1.0 {
		t.Errorf("user = %v (top still parsed)", c.UserPct)
	}
}

func TestParseCPUUsage_Variants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		line  string
		check func(*testing.T, float64, float64, float64)
	}{
		{
			name: "live macos26 line",
			line: "CPU usage: 20.85% user, 16.25% sys, 62.88% idle",
			check: func(t *testing.T, user, sys, idle float64) {
				if user != 20.85 || sys != 16.25 || idle != 62.88 {
					t.Errorf("got %v/%v/%v", user, sys, idle)
				}
			},
		},
		{
			name: "trailing punctuation",
			line: "CPU usage: 5.10% user, 3.20% sys, 91.70% idle.",
			check: func(t *testing.T, user, sys, idle float64) {
				if user != 5.10 || idle != 91.70 {
					t.Errorf("got %v/%v/%v", user, sys, idle)
				}
			},
		},
		{
			name: "garbage line",
			line: "this is not top output",
			check: func(t *testing.T, user, sys, idle float64) {
				if user != 0 || sys != 0 || idle != 0 {
					t.Errorf("expected zeros; got %v/%v/%v", user, sys, idle)
				}
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			user, sys, idle := system.ParseCPUUsage(c.line)
			c.check(t, user, sys, idle)
		})
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
