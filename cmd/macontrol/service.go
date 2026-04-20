package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const (
	plistLabel = "com.amiwrpremium.macontrol"
	plistName  = "com.amiwrpremium.macontrol.plist"
)

func runService(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: macontrol service {install|uninstall|start|stop|status|logs}")
		os.Exit(2)
	}
	switch args[0] {
	case "install":
		if err := serviceInstall(); err != nil {
			fatalf("service install: %v", err)
		}
		fmt.Println("installed.")
	case "uninstall":
		if err := serviceUninstall(); err != nil {
			fatalf("service uninstall: %v", err)
		}
		fmt.Println("uninstalled.")
	case "start":
		if err := serviceStart(); err != nil {
			fatalf("service start: %v", err)
		}
	case "stop":
		if err := serviceStop(); err != nil {
			fatalf("service stop: %v", err)
		}
	case "status":
		serviceStatus()
	case "logs":
		serviceLogs()
	default:
		fatalf("unknown service subcommand: %s", args[0])
	}
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func serviceInstall() error {
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	body := plistBody(binary, filepath.Join(home, "Library", "Logs", "macontrol"))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	return serviceStart()
}

func serviceUninstall() error {
	_ = serviceStop()
	path := plistPath()
	return os.Remove(path)
}

func serviceStart() error {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "bootstrap", "gui/"+uid, plistPath())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func serviceStop() error {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "bootout", "gui/"+uid+"/"+plistLabel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func serviceStatus() {
	uid := strconv.Itoa(os.Getuid())
	cmd := exec.Command("launchctl", "print", "gui/"+uid+"/"+plistLabel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func serviceLogs() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "Logs", "macontrol", "macontrol.log")
	cmd := exec.Command("tail", "-n", "200", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
