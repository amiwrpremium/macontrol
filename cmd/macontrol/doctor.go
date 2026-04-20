package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

var brewDeps = []struct {
	Bin  string
	Why  string
	Hint string
}{
	{"brightness", "💡 brightness control", "brew install brightness"},
	{"blueutil", "🔵 Bluetooth toggle/list/connect", "brew install blueutil"},
	{"terminal-notifier", "🔔 rich notifications (fallback: osascript)", "brew install terminal-notifier"},
	{"smctemp", "🌡 °C readings on Apple Silicon", "brew install smctemp"},
	{"imagesnap", "📸 webcam photos", "brew install imagesnap"},
}

func runDoctor() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := runner.New()

	fmt.Printf("macontrol %s (%s, %s)\n", version, commit, date)
	fmt.Printf("runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS != "darwin" {
		fmt.Println("⚠ macontrol targets darwin/arm64. Only a subset of checks will run here.")
	}

	if rep, err := capability.Detect(ctx, r); err == nil {
		fmt.Println()
		fmt.Printf("macOS:            %s\n", rep.Version)
		fmt.Printf("networkQuality:   %v\n", rep.Features.NetworkQuality)
		fmt.Printf("shortcuts CLI:    %v\n", rep.Features.Shortcuts)
		fmt.Printf("wdutil info:      %v\n", rep.Features.WdutilInfo)
	} else {
		fmt.Printf("⚠ could not run sw_vers: %v\n", err)
	}

	fmt.Println()
	fmt.Println("optional brew deps:")
	for _, d := range brewDeps {
		mark := "✓"
		if _, err := exec.LookPath(d.Bin); err != nil {
			mark = "✗"
		}
		fmt.Printf("  %s %-18s %s\n     └ %s\n", mark, d.Bin, d.Why, d.Hint)
	}

	fmt.Println()
	fmt.Println("sudoers (pmset):")
	if _, err := r.Sudo(ctx, "pmset", "-g"); err == nil {
		fmt.Println("  ✓ `sudo -n pmset -g` works")
	} else {
		fmt.Println("  ✗ `sudo -n pmset -g` failed — install sudoers entry via `macontrol setup`")
	}
}
