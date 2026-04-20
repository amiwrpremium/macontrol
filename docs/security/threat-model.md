# Threat model

What an attacker can do at each level of access. Useful for thinking
about what to protect, what's already locked down, and what's
intentionally not protected (because it's not in scope).

## Levels of access

We map five increasing levels:

1. **Internet drive-by** — knows the bot exists, no credentials.
2. **On the whitelist** — has a Telegram account whose ID is on
   `ALLOWED_USER_IDS`. Either added by you intentionally, or via a
   compromise that altered the config file.
3. **Has the bot token** — can send and receive on the bot, but their
   Telegram account is not whitelisted.
4. **Filesystem read** on your Mac — can read `~/Library/Application
   Support/macontrol/config.env`.
5. **Filesystem write + your shell** — full account compromise.

Each level subsumes the ones above (level 5 has all the powers of
levels 1–4 plus its own).

## Level 1 — Internet drive-by

**Capabilities**:
- Know that you have a macontrol bot (if they DM it and see no
  response, they can't tell whether the bot is silent or doesn't exist).
- Add your bot to a group they're in. Bot will see group messages
  but won't act on them (whitelist drops everything).

**Mitigations already in place**:
- Bot is silent to non-whitelisted users — no enumeration that the
  bot is real or that macontrol is running.
- Long-poll, no public endpoint — there's no port to scan.

**Residual risk**: low. Telegram doesn't expose your username via the
bot's API, so they can't easily figure out your contact info from the
bot alone.

## Level 2 — On the whitelist

**Capabilities**:
- Every action a legitimate user can perform: lock, sleep, restart,
  shutdown, change DNS, take screenshots and recordings, use webcam,
  send notifications, set timezone, run any user Shortcut, etc.
- Everything in [Usage → Categories](../usage/categories/README.md).

**What this means concretely**:
- They can shut your Mac down right now. (You can't undo via Telegram
  after that — you'd need physical access to power it back on.)
- They can take a screenshot of whatever's on your screen, including
  banking sessions, password managers, anything visible.
- They can record short videos (up to ~5 s practical given upload
  limits).
- They can take a webcam photo.
- They can change your DNS to a server they control, opening MITM on
  your browser traffic.
- They can run any user-authored Shortcut, which could itself
  encompass more (HomeKit, file manipulation via Shortcuts actions,
  iCloud queries, etc.).

**They CANNOT** (without elevating further):
- Read or modify files on your Mac other than what those actions
  expose.
- Run arbitrary shell commands. There's no `/sh`.
- Install software.
- Access the bot token (it's not exposed via the bot's commands).
- Pivot to other Macs.

**Mitigations**:
- Confirmation step on destructive actions (Restart / Shutdown /
  Logout). Two taps, not one.
- Audit log captures every action with the user ID — you can review
  what they did after the fact.

**Residual risk**: significant. The whitelist is one tier — it's
trusted access, not least-privileged. Don't add people you don't
trust with full control of your Mac.

## Level 3 — Has the bot token

**Capabilities**:
- Send messages from your bot.
- Receive every Telegram update sent to your bot (via long-poll).
- Edit and delete the bot's messages.
- See usernames and IDs of anyone who messages the bot.

**They CANNOT**:
- Trigger any macOS action — they're not on your whitelist, and
  Telegram's authentication binds messages to real user accounts
  (which they don't have).
- Read your config or files.
- Change the whitelist (would require Mac access).

**Damage they can do**:
- **Spam your DMs** with the bot. Your phone keeps getting "macontrol"
  notifications. Annoying.
- **Confuse you** — send a message that looks like it's from your
  bot, like "macOS update available, please tap here". Phishing-ish.
- **Read what you sent the bot** — your full chat history is visible
  to them.

**Mitigations**:
- Whitelist enforcement — without your user ID, they can't trigger
  actions.
- BotFather token revocation is one DM and 30 seconds.

**Detection**: if the long-poll is being consumed elsewhere, your
real bot might miss updates or get them duplicated. You'd notice as
flakiness. Not a great detection signal.

**Residual risk**: medium. Token leak is annoying but not
catastrophic if your whitelist is locked down.

## Level 4 — Filesystem read

**Capabilities** (everything from levels 1–3, plus):

- Read `config.env`, get the bot token.
- Read `ALLOWED_USER_IDS` and know what user IDs you trust.
- Read logs — see every action you've taken via the bot, with
  timestamps.
- Read any other readable file — your terminal history, browser data,
  SSH keys, etc.

**Damage they can do**:
- Combine the token with their own Telegram account on the whitelist
  → full level-2 access. To do this they'd need to also write to
  config (level 5), but if they can read everything else, they probably
  already have your account credentials and can grab any other
  service's tokens too.
- Plant credentials elsewhere by knowing what's on the whitelist.

**Mitigations**:
- File mode `0600` — only your Unix user can read. Other users on
  the same Mac are blocked.
- FileVault encryption protects against offline disk access (USB
  boot, drive removal).

**Residual risk**: high. By the time someone has read access to your
home directory, they probably have everything else they need too.

## Level 5 — Filesystem write + shell

**Capabilities**: everything. They can:

- Modify the whitelist to add their own ID.
- Modify the bot token (e.g. point at their own bot).
- Disable the daemon entirely.
- Plant a backdoor in the binary or in a brew formula.
- Read every file you can read.

**Mitigations**: at this point macontrol can't help you. You've been
fully compromised. The defense is general macOS account security
(strong password, FileVault, careful what you `sudo` install, etc.).

**Residual risk**: catastrophic. macontrol is a small part of what's
been lost.

## What macontrol *intentionally* doesn't protect against

- **Physical access to an unlocked Mac**. macontrol's whole purpose
  is to let *you* control your Mac remotely; an attacker with the
  Mac unlocked in front of them already has more direct ways to do
  damage.
- **Compromise of Telegram's infrastructure**. We trust Telegram's
  authentication of users.
- **Compromise of brew formulae** for `brightness`, `blueutil`, etc.
  If those are tampered with, they're subprocess shells running with
  your user's permissions.
- **Side-channel attacks** on the daemon itself (timing, memory). Out
  of scope for a single-user bot.
- **Denial of service** by sending many requests. The whitelist drops
  non-allowed senders cheaply, but a whitelisted attacker could spam
  the bot and cause subprocess thrashing. The flow registry's TTL
  prevents flow-state exhaustion.

## Recommended posture

For a normal solo user:

1. **Use the narrow sudoers entry**, not blanket sudo.
2. **Whitelist exactly the user IDs you need** — no spares "in case".
3. **Don't paste the token into chats, paste-bins, screenshots, or
   commit it to git** (even private repos).
4. **Keep your Mac's account password strong** and FileVault on.
5. **Watch the boot ping** — if it stops arriving when the daemon
   should be running, investigate. If it arrives at unexpected times,
   investigate.
6. **Review logs occasionally** — `tail -100 ~/Library/Logs/macontrol/macontrol.log`
   is a 30-second sanity check.

For a higher-threat user (security researcher, journalist, public
figure):

7. **Rotate the bot token quarterly** (`/revoke` in BotFather, update
   config, restart).
8. **Run on a dedicated user account** that doesn't have admin rights
   itself.
9. **Audit `sudoers.d/macontrol` content matches the template** —
   defense against tampering.
10. **Pin the bot to a single chat** (yourself, no group inclusions)
    so you can spot stray messages.

## Out of scope

- **Multi-tenant access controls** — not designed for it.
- **End-to-end encrypted messages** — Telegram bot messages are
  client-server encrypted, not E2E. If E2E matters to you, this isn't
  your tool.
- **Defense against your IT department's MDM** — a managed Mac can
  be controlled by your org regardless of what macontrol does.
- **Defense against macOS itself** — if Apple ships an update that
  breaks the trust model (extremely unlikely), there's no workaround.

## Reporting issues

If you find a vulnerability — anything that lets a lower-level
attacker do something they shouldn't — see
[Reporting vulnerabilities](reporting-vulnerabilities.md). Use private
disclosure; don't open a public issue with reproduction details.
