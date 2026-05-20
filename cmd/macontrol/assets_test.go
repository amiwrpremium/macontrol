package main

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestPlistBody_IsValidXML(t *testing.T) {
	body := plistBody("/opt/homebrew/bin/macontrol", "/Users/x/Library/Logs/macontrol")
	var out struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("plist not valid XML: %v", err)
	}
	if !strings.Contains(body, plistLabel) {
		t.Errorf("missing label")
	}
	if !strings.Contains(body, "/opt/homebrew/bin/macontrol") {
		t.Errorf("missing binary path")
	}
	if !strings.Contains(body, "/Users/x/Library/Logs/macontrol/macontrol.log") {
		t.Errorf("missing log path")
	}
}

func TestPlistBody_KeepAliveAndRunAtLoad(t *testing.T) {
	body := plistBody("/x", "/y")
	for _, want := range []string{"RunAtLoad", "KeepAlive", "StandardOutPath", "StandardErrorPath", "EnvironmentVariables"} {
		if !strings.Contains(body, want) {
			t.Errorf("plist missing %q key", want)
		}
	}
}

func TestSudoersBody_ContainsAllFiveBinaries(t *testing.T) {
	body := sudoersBody()
	for _, bin := range []string{"pmset", "shutdown", "wdutil", "powermetrics", "systemsetup"} {
		if !strings.Contains(body, bin) {
			t.Errorf("sudoers missing %q", bin)
		}
	}
}

func TestSudoersBody_NoPassword(t *testing.T) {
	body := sudoersBody()
	if !strings.Contains(body, "NOPASSWD") {
		t.Error("sudoers entry should specify NOPASSWD")
	}
}

func TestSudoersBody_NotBlanket(t *testing.T) {
	body := sudoersBody()
	if strings.Contains(body, "ALL=(ALL) NOPASSWD: ALL") {
		t.Error("sudoers entry must not grant blanket ALL")
	}
}
