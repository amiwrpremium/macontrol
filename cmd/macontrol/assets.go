package main

import "fmt"

// sudoersBody renders the narrow `/etc/sudoers.d/macontrol`
// file content for the current user. Matches the
// sudoers.d/macontrol.sample template at the repo root.
//
// The file grants passwordless `sudo -n` access to exactly
// five binaries — no blanket `ALL=(root) NOPASSWD: ALL`. Each
// binary corresponds to one feature category that the daemon
// needs:
//
//   - /usr/bin/pmset       → Power → sleep / displaysleepnow.
//   - /usr/sbin/shutdown   → Power → shutdown / restart paths
//     (currently AppleScript-driven, but kept for fallback).
//   - /usr/bin/wdutil info → Wi-Fi → SSID + diagnostics dump.
//   - /usr/bin/powermetrics → System → thermal-pressure sample.
//   - /usr/sbin/systemsetup → Tools → timezone read/list/set.
//
// Returns the rendered file contents. Used by
// [installSudoersFile] in setup.go (which writes the result
// via `sudo install -m 0440`).
//
// The current Unix user is templated in via [currentUser]
// rather than hardcoded so the file works regardless of
// which user runs `macontrol setup`.
func sudoersBody() string {
	return fmt.Sprintf(`# /etc/sudoers.d/macontrol
# Narrow passwordless-sudo entry for the macontrol daemon.
# Only the five binaries the bot actually needs; no blanket ALL.

%s  ALL=(root) NOPASSWD: /usr/bin/pmset
%s  ALL=(root) NOPASSWD: /usr/sbin/shutdown
%s  ALL=(root) NOPASSWD: /usr/bin/wdutil info
%s  ALL=(root) NOPASSWD: /usr/bin/powermetrics
%s  ALL=(root) NOPASSWD: /usr/sbin/systemsetup
`, currentUser(), currentUser(), currentUser(), currentUser(), currentUser())
}

// plistBody renders a LaunchAgent plist that runs
// `<binary> run` at login and restarts on crash.
//
// Arguments:
//   - binary is the absolute path to the macontrol binary.
//     Stamped verbatim into ProgramArguments — the LaunchAgent
//     references this exact path forever, so a binary that
//     moves later (brew upgrade relocate, manual mv) breaks
//     the agent until it's regenerated via
//     `macontrol service install` or `token reauth` + plist
//     rewrite. See the smells noted on service.go.
//   - logDir is the directory for stdout/stderr log files;
//     macontrol.log and macontrol.err.log are created
//     there, with size-based rotation handled inside the
//     daemon (see [newLogger] and lumberjack).
//
// Behavior:
//   - <Label> uses [plistLabel] as the launchd identifier —
//     same constant the bootstrap/bootout commands use.
//   - <RunAtLoad>true</RunAtLoad> + <KeepAlive>true</KeepAlive>
//     means launchd starts the daemon at user login and
//     restarts it on any exit.
//   - <ProcessType>Interactive</ProcessType> hints to
//     launchd that we expect to be active when the user is
//     logged in (vs Background/Adaptive/Standard).
//   - <EnvironmentVariables> overrides PATH so the daemon
//     finds homebrew binaries (brightness, blueutil, smctemp,
//     etc.) regardless of how launchd composed its own PATH.
//     Both /opt/homebrew/bin (Apple Silicon brew prefix) and
//     /usr/local/bin (Intel brew prefix) are included so the
//     plist works on both architectures.
//
// Returns the rendered XML string ready to be written via
// [serviceInstall].
func plistBody(binary, logDir string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>StandardOutPath</key>
    <string>%s/macontrol.log</string>
    <key>StandardErrorPath</key>
    <string>%s/macontrol.err.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
`, plistLabel, binary, logDir, logDir)
}
