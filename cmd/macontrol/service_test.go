package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistPath_UsesHomeLaunchAgents(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := plistPath()
	want := filepath.Join(tmp, "Library", "LaunchAgents", plistName)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !strings.HasSuffix(got, ".plist") {
		t.Errorf("expected .plist suffix")
	}
}

func TestPlistLabel_Canonical(t *testing.T) {
	if plistLabel != "com.amiwrpremium.macontrol" {
		t.Errorf("plist label = %q", plistLabel)
	}
}
