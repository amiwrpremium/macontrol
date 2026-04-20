# Security

macontrol gives a Telegram chat the ability to control your Mac. That
power has to be matched with care about what's protected and what's
exposed.

## What's here

- **[Bot token](bot-token.md)** — the secret class table, why the
  token matters, rotation playbook.
- **[Threat model](threat-model.md)** — what an attacker with access
  at each level can actually do.
- **[Reporting vulnerabilities](reporting-vulnerabilities.md)** — the
  private-disclosure channel.

## At a glance

The two pieces of secret material:

| Secret | Class | Stored where |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | High — anyone with it can act as your bot | `~/Library/Application Support/macontrol/config.env` (mode 0600) |
| `ALLOWED_USER_IDS` | Low — reveals who's allowed, not how to access | Same file. Not really a secret. |

The two enforcement layers:

| Layer | Purpose |
|---|---|
| Telegram user-ID whitelist | Drops every update from a non-whitelisted sender silently |
| Narrow sudoers entry | Limits sudo access to five named binaries (no `sudo ALL`) |

The two TCC-managed permissions:

| TCC bucket | What it grants |
|---|---|
| Screen Recording | Capture displays |
| Camera | Capture webcam |

(Plus Accessibility for app listing, Automation for AppleScript.)

## Posture in one sentence

macontrol is **single-owner-by-design**: one whitelist, one bot, one
Mac. There's no role-based access, no per-user audit log beyond the
Telegram user ID written to logs, and no encryption-at-rest beyond
what macOS gives you for free (FileVault).

Anything that erodes those defaults — sharing the token, expanding the
whitelist, granting blanket sudo, weakening TCC grants — increases the
blast radius of a compromise. Read [Threat model](threat-model.md) for
the specifics of what each level of compromise enables.
