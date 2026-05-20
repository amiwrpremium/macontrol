// Command macontrol is the single binary shipped to macOS. It runs the
// Telegram bot daemon (`macontrol run`) and also provides the
// `setup`, `service`, `doctor`, `whitelist`, and `token` helpers used by
// the Homebrew formula and the manual-install script.
//
// Subcommands are thin CLI wrappers around the [internal] packages:
// `run` starts the long-lived bot, while the rest are one-shot admin tools
// that configure the Keychain, user whitelist, LaunchAgent, and sudoers
// entries. The binary is self-contained — no external assets, no shell
// scripts — so it can be distributed as a Homebrew bottle or a notarised
// tarball and still bootstrap itself from `macontrol setup`.
package main
