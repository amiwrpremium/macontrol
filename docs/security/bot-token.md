# Bot token

The single most sensitive piece of macontrol's configuration. Treat
it like a password.

## What it is

Format: `<bot-id>:<secret>`. Example:

```text
123456789:AAE-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456
```

The numeric prefix is the bot's Telegram ID (public — visible to
anyone the bot has interacted with). The secret after the colon is
the token: a 35-character base64-style string that authenticates HTTP
requests to `api.telegram.org/bot<token>/...`.

## What an attacker can do with it

**Without your user ID** on their whitelist:

- **Send messages from your bot** to any chat the bot is in. They
  can spam your DMs, post to groups the bot is in, etc.
- **Read incoming updates** to your bot via long-poll. They see every
  message anyone sends to your bot, including yours.
- **Edit / delete messages your bot sent**.
- **Change the bot's profile** (name, avatar, description) via
  BotFather... wait, no — that requires DM access to BotFather, not
  just the token.
- **Cannot trigger any macOS action**. The whitelist on macontrol
  rejects them.

**With your user ID** also (e.g. they have your token AND a copy of
the whitelist Keychain entry):

- They could spoof messages to your bot, but they can't actually be
  on your Telegram account, so the messages would come from the
  attacker's account. They'd still need to be on your whitelist.

In practice: the token alone gives them bot-side mischief but no
control over your Mac. The combination of token + Mac access (e.g.
remote access to your filesystem to read the config) gives them full
control.

## How to revoke

1. DM [@BotFather](https://t.me/BotFather).
2. Send `/revoke`.
3. BotFather lists your bots. Pick the one whose token leaked.
4. BotFather replies with a fresh token. The old one is dead within
   seconds — every API call with the old token returns
   `Unauthorized`.

After revoking:

```bash
# Replace the token in the Keychain (interactive, hidden input)
macontrol token set

# Restart the daemon
brew services restart macontrol
# or
macontrol service stop && macontrol service start
```

The bot will resume working with the new token immediately.

## When to revoke

Suspect compromise — revoke first, ask questions later. Specific
triggers:

- You pasted the token into a chat, log, or screenshot you can't fully
  control.
- You committed any file containing the token to git, even a
  private repo (assume it's compromised).
- You used the token from a non-trusted machine.
- A grep of git history finds it: `git log -p | grep -E '[0-9]{9,10}:[A-Za-z0-9_-]{30,}'`.
- You see logged messages from chats you didn't initiate.
- An old hardware device is lost / stolen and might have had the
  config file.

Revocation has zero cost — no users are affected, no formulae break.
The only friction is updating the config and restarting.

## Rotation policy

If you have an active threat model (e.g. you're a security researcher,
journalist, etc.), rotate every 90 days. For most solo users, rotate
when there's a reason; otherwise leave it.

## How macontrol stores it

The token lives in your **macOS login keychain** under service name
`com.amiwrpremium.macontrol`, account = your unix username. The
underlying database is at:

```text
~/Library/Keychains/login.keychain-db
```

Encrypted at rest with your account password (independent of FileVault).
Per-app silent-read ACL granted to the macontrol binary by the setup
wizard — other readers (including you running `security
find-generic-password` from a terminal that wasn't pre-authorised)
trigger a one-time interactive prompt that asks "Always Allow?".

Inspect with:

```bash
security find-generic-password -s com.amiwrpremium.macontrol -a $USER -w
```

### What an attacker who reads `~/` cannot do

Unlike a plaintext `.env` file, an attacker who somehow gets read access
to your home directory still can't extract the token without your
account password — the keychain database is encrypted and the
entry-level decryption requires either:

- the macontrol binary running with the right ACL (interactive prompt
  for any other process), OR
- your account password (via `security unlock-keychain`).

This is a meaningful upgrade over file mode 0600, which is "anyone in
your unix user can `cat` it".

### Binary-move re-prompts (without code signing)

The Keychain ACL is binary-path-based until macontrol gets a Developer
ID code signature (deferred to v1.x). If you reinstall to a different
path (brew bottle relocation, switching brew ↔ manual install), the
ACL invalidates and macOS re-prompts on next read.

Recovery without re-entering the token:

```bash
macontrol token reauth
```

Re-issues the Keychain entry with the new binary path. Silent reads
resume.

## What macontrol does NOT do

- **No log redaction** — macontrol's logger doesn't write the token
  out. But: `--log-level=debug` could log Telegram API request URLs
  that include the token in `<token>` path segments. Don't run
  debug-level logging long-term, and grep before sharing logs.
- **No environment-variable input** — the daemon reads only from the
  Keychain at startup. There is no `TELEGRAM_BOT_TOKEN` env var
  override.
- **No `.env` file** — the token is never written to disk in
  plaintext by macontrol.
- **No iCloud Keychain sync** — the entry is created locally only.
  Tokens shouldn't sync to other devices since they're tied to a
  specific Mac's daemon.

## Token in transit

macOS → Telegram is over TLS to `api.telegram.org`. The Go HTTP
client uses the system's trust store (`/System/Library/Keychains/...`).
No TLS pinning — if your CA store is compromised, MITM is possible,
but that's a deeper compromise.

The token is in the URL path of every request:

```text
POST https://api.telegram.org/bot<TOKEN>/sendMessage
```

This means it appears in any HTTP debug logs or proxy access logs you
might have running. Don't man-in-the-middle your bot's traffic
without thinking about that.

## What about multi-bot setups

If you want to run a dev bot and a prod bot from the same Mac:

- Create two bots with BotFather, get two tokens.
- Run each daemon as a different macOS user account. Each user has
  its own login keychain, so the two tokens stay isolated. Each
  user gets its own LaunchAgent plist and log directory.

Don't share tokens across daemons; if one leaks, the other is
unaffected.

## Compromised tokens — incident response

The minimum playbook:

1. **Revoke** via BotFather (`/revoke`). 30 seconds.
2. **Restart daemon** with new token. 1 minute.
3. **Audit logs** — `~/Library/Logs/macontrol/macontrol.log` from the
   suspected compromise window. Look for unfamiliar Telegram user IDs
   on the whitelist (which would mean the attacker also wrote to your
   Keychain — much deeper compromise) or unfamiliar action patterns.
4. **Reverse anything destructive** — DNS changes, sudo configurations,
   anything in `/etc/`.
5. **If filesystem-level compromise is suspected**: assume your sudoers
   entry, your shell history, and your other credentials are also
   compromised. Replace them all.

## Token != identity

A common confusion: the bot token doesn't authenticate **users**, only
the bot itself. Telegram delivers updates from real Telegram users to
the bot via the long-poll, and the bot trusts whatever sender ID
Telegram puts on each update. The bot doesn't authenticate the user
beyond that — the whitelist is purely "did Telegram tell us this
user ID?".

Implication: anything that can spoof Telegram's delivery (which
nothing in the wild can today) would bypass macontrol's whitelist.
We trust Telegram's authentication of users; the trust is one-hop.
