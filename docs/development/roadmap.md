# Roadmap

What's planned, what's actively being considered, and what's
explicitly out of scope. Living document — updated when priorities
shift.

## Soon (next 1–2 releases)

These are concrete and actionable.

### Apple Developer ID signing + notarization

The current binary ships unsigned. Gatekeeper quarantines it (users
have to `xattr -d com.apple.quarantine` once before the binary will
run). Signing + notarization removes that friction.

Cost: ~$99/year for an Apple Developer account.

Implementation:

- Sign in `release.yml` after `goreleaser build` and before
  `goreleaser publish`.
- GoReleaser has built-in support for `codesign` + `notarytool`.
- Secrets needed: signing certificate (P12 + password), Apple ID +
  app-specific password for notarization.

### Lock the coverage floor

`.testcoverage.yml` exists but the CI step is `continue-on-error:
true`. Once a few releases of CI history confirm coverage is stable
at the current level, flip it to blocking.

### Boot-ping opt-out

Some users find the boot ping noisy. Add an env var
`BOOT_PING=false` to suppress it.

### `macontrol setup --token` flag

For automated installs, add a non-interactive mode where the token
is supplied via flag or env var.

## Considering (would accept good PRs)

### External monitor brightness via DDC

`ddcctl` integration for external monitor brightness. Opt-in (a config
flag) since not every monitor supports DDC and macontrol's UX is
built around the built-in display.

### Spotify / Music control

`📷 Media → 🎵 Now playing` showing the current track from
Spotify/Apple Music with play/pause/skip buttons. Backed by AppleScript
to the respective app.

### Network info card

`📶 Wi-Fi → Card` with a richer info dashboard: signal strength bars,
channel chart, link rate. Wraps `wdutil info` parsing.

### Process search

`🖥 System → Find process…` flow that searches `ps` output for a
substring and returns matching processes (PID + command).

### Custom keyboard layouts

Allow users to configure which categories show on the home keyboard
and in what order, via a YAML in the config dir. Useful if you only
use a subset of features.

### Webhook mode (alternative to long-poll)

Optional support for Telegram webhooks instead of long-polling. Would
require an HTTPS endpoint reachable from Telegram's IPs — users would
need a domain + cert + reverse proxy. Niche, but might be useful for
some self-hosters.

## Maybe / open questions

### Multi-Mac support

A single bot controlling multiple Macs. Each Mac would have its own
daemon; the bot would route commands to the right one.

Open questions:

- How does the user pick which Mac in a chat?
- Does each Mac have its own whitelist?
- Is this really useful, or are most users single-Mac?

Probably out of scope unless there's clear demand.

### iOS shortcuts integration

A Shortcut on the user's iPhone that invokes macontrol actions via
Telegram's `sendMessage` API. Bypasses opening the bot manually.

Could be a docs-only contribution (here's a Shortcut JSON to import)
rather than code.

### Voice mode

Telegram supports voice messages. Could parse speech to determine the
intended command. Heavy dependency on speech recognition; probably
not worth the complexity for a tool that's already keyboard-driven.

## Explicit non-goals

These have been considered and **explicitly rejected**. Don't open PRs
for them — they'll be closed.

### Intel Mac support

Already covered in [Architecture → Design decisions](../architecture/design-decisions.md#why-apple-silicon-only).

### Arbitrary `/sh <cmd>` escape hatch

Already covered in [Design decisions](../architecture/design-decisions.md#why-named-commands-only--no-sh-escape-hatch).
The Shortcuts runner is the deliberate alternative.

### Multi-user roles / RBAC

macontrol is single-tier-whitelist by design. Adding owner/operator/viewer
roles is heavyweight for a single-owner tool. Not happening.

### Linux/Windows ports

The whole codebase is macOS-specific. Porting would mean rewriting
every domain package. If you want a similar tool for Linux, look at
existing tools like `ts-bot` or roll your own — macontrol's
architecture is reusable as a template, but the code isn't.

### Web UI / phone app

Telegram is the UI. Adding a parallel UI would mean maintaining two
surfaces. Not happening.

### File-system browsing

A `📁 Files` category for browsing your Mac's filesystem from
Telegram. The combination of Telegram message size limits + the
attack-surface implications of arbitrary file reads makes this a
poor fit.

If you want remote file access, use SSH or a proper file-sync tool.

### Real-time pushes for state changes

The bot is request-response, not push. Subscribing to "tell me when
the battery hits 20%" would mean a per-chat polling loop. macontrol
is intentionally idle when nobody's looking.

For battery alerts and similar, use the macOS Shortcuts app's
personal automations.

### A web installer

A "click here to install" experience would need a code-signed installer
package. Not a good fit for an open-source CLI tool. The brew /
curl-script paths are the install paths.

### Config UI

A web UI to edit `config.env`. Adds a web server and an attack
surface for very little gain — `vim ~/Library/Application
Support/macontrol/config.env` is fine.

## How to propose changes

For something on the **Considering** list: open a PR, link to this
doc.

For something not listed: open a Discussion first to gauge interest.
If maintainer agrees it fits the project, an issue gets opened and
this doc updated.

For something on the **Non-goals** list: don't open a PR. Open a
Discussion if you want to argue the case for re-evaluating, but the
default answer is no.

## Velocity

Solo-maintained — releases when there's something to release.
Roadmap items are best-effort, not commitments.
