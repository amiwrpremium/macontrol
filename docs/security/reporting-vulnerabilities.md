# Reporting vulnerabilities

If you've found a security issue in macontrol, please report it
**privately** before disclosing publicly.

## Where

Use GitHub's private vulnerability reporting:

<https://github.com/amiwrpremium/macontrol/security/advisories/new>

This creates a draft security advisory visible only to the maintainer
(@amiwrpremium) until disclosure.

If you can't use GitHub's UI, email **amiwrpremium@gmail.com** with
the subject line `[macontrol-security]`.

## What to include

A good report makes the difference between a same-day fix and a
multi-week back-and-forth.

- **Macontrol version** — `macontrol --version` output.
- **macOS version + chip** — `sw_vers -productVersion` and
  `sysctl -n machdep.cpu.brand_string`.
- **Description of the issue** — what's vulnerable, in plain English.
- **Threat-model context** — at which access level can this be
  exploited? See [Threat model](threat-model.md).
- **Reproduction steps** — exact commands, expected vs actual
  behavior.
- **Proof of concept** — a minimal demonstration. Code snippet,
  command transcript, or screen recording.
- **Suggested fix** — if you have one. Optional but appreciated.

If you've already published the vulnerability, say so up-front so
the response prioritizes accordingly.

## What's in scope

- The macontrol daemon and CLI binary.
- The LaunchAgent plist and sudoers template shipped with releases.
- The Homebrew formula in `amiwrpremium/homebrew-tap`.
- The `scripts/install.sh` bootstrap.
- The setup wizard's interactive flows.
- The Telegram protocol implementation (callback validation, whitelist
  enforcement, flow lifecycle).

## What's out of scope

- **Issues requiring already-elevated access** — having the bot token
  is full bot control by design; having user-ID-on-whitelist is full
  Mac control by design. See [Threat model](threat-model.md).
- **Vulnerabilities in upstream dependencies** — `go-telegram/bot`,
  `lumberjack`, `imagesnap`, `blueutil`, etc. Report those to the
  upstream project. We'll bump our pin once they ship a fix.
- **Vulnerabilities in macOS CLIs** (`pmset`, `networksetup`,
  `screencapture`, etc.) — report to Apple via `https://security.apple.com`.
- **Social-engineering of the user** — convincing someone to install
  a fake bot or paste a malicious token isn't a macontrol vulnerability.

## Disclosure timeline

What to expect:

- **0–72 hours**: acknowledgment that the report was received.
- **3–7 days**: initial assessment. Either confirmed-and-tracked or
  rejected-with-reason.
- **30 days**: target for a patch in master. May take longer for
  complex issues.
- **30–90 days**: coordinated disclosure. We'll publish a security
  advisory and CHANGELOG entry. Credit you (with your preferred
  handle) unless you ask not to be named.

We're a single-maintainer project — there's no SLA, but we take
security seriously and aim for these timelines.

## What we won't do

- **Pay bug bounties** — no budget for a reward program.
- **Sign NDAs** — disclosure terms above are public and apply equally
  to all reporters.
- **Suppress disclosure** — once a fix is shipped, the security
  advisory is published, even if the reporter requests otherwise. Open
  source shouldn't have hidden vulnerabilities in shipped releases.

## Public CVE process

For severity-significant issues (CVSS ≥ 7), we'll request a CVE via
GitHub's CNA process when publishing the advisory. Lower-severity
issues are documented in the CHANGELOG and the GitHub Security
Advisories tab without a CVE.

## Coordinated vs. uncoordinated disclosure

- **Coordinated**: you report privately, we fix and release, you
  publish your write-up referencing the patched version. Preferred.
- **Uncoordinated** (90+ days with no patch): if we've gone radio
  silent or refuse to fix, you're free to disclose publicly. Please
  give us a heads-up first via email.

The short version: be a good citizen, give us a chance to fix it,
and we'll do the same for you.
