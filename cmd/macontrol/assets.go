package main

import "fmt"

// sudoersBody returns the narrow sudoers file content. Matches
// sudoers.d/macontrol.sample in the repo.
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

// plistBody returns a LaunchAgent plist that runs `macontrol run` and writes
// stdout/stderr to logDir.
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
