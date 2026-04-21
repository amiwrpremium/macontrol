# Whitelist

The whitelist lives in the macOS Keychain (service
`com.amiwrpremium.macontrol.whitelist`) — see
[Configuration → Runtime](runtime.md#where-everything-lives).
Manage it via the `macontrol whitelist` subcommands (no file editing
needed):

```bash
macontrol whitelist list
macontrol whitelist add 987654321
macontrol whitelist remove 987654321
macontrol whitelist clear      # asks for confirmation
```

After any add/remove, restart the daemon for it to take effect:

```bash
brew services restart macontrol
```

The rest of this doc explains the access model.

---

macontrol uses a hard, single-tier whitelist — every user on the list
gets full access, every other Telegram user is silently ignored.

## How it works

Every incoming Telegram update (message, callback query, etc.) is
inspected for the sender's numeric Telegram user ID. The daemon looks
that ID up in an in-memory `map[int64]struct{}` built at startup
from the Keychain whitelist entry.

- **Hit** → dispatch the update normally.
- **Miss** → drop silently. A single warning is logged with the
  rejected sender ID for forensic purposes.

The unauthorized user gets no reply. From their perspective, the bot
appears dead. This is intentional — replying to non-whitelisted users
("you don't have access") would let an attacker enumerate that the bot
exists and is functioning.

## Adding a user

### Get their numeric ID

The user has to look it up themselves on their account:

1. Open Telegram on the device they'll use.
2. Search for [@userinfobot](https://t.me/userinfobot).
3. Send `/start`.
4. Read the `Id:` line. They send you that number.

Numeric IDs are permanent — they don't change if the user renames their
@username or signs in on a different device. The same user on phone +
desktop has the same ID (one ID per Telegram account).

### Add via the CLI

```bash
macontrol whitelist add 987654321
```

The command checks that the value parses as an integer, deduplicates
against existing entries, and writes back to the Keychain. Output:

```text
✓ added 987654321 (whitelist now has 2 entries)
restart the daemon for changes to take effect: brew services restart macontrol
```

If you'd rather rebuild the whole list from scratch, run
`macontrol setup --reconfigure` and re-enter the comma-separated list.

### Restart the daemon

```bash
brew services restart macontrol
# or
macontrol service stop && macontrol service start
```

The daemon doesn't hot-reload — config is read once at startup.

### Verify

The new user sends `/menu` to the bot. They should see the welcome +
home keyboard. If they get no reply, the daemon either didn't restart
or didn't pick up the new ID.

```bash
# Confirm the daemon is running with the updated whitelist
tail -f ~/Library/Logs/macontrol/macontrol.log
# Look for: "macontrol starting" with the current PID
```

## Removing a user

```bash
macontrol whitelist remove 987654321
```

Then restart the daemon. Their next message gets dropped silently.

The CLI refuses to remove the **last** entry as a safety check
(otherwise the daemon would refuse to start at all). Use `macontrol
whitelist clear` if you genuinely want to empty the list — it asks
for explicit confirmation.

There's no "soft remove" — once they're off the list, they have no
access. They can still see the chat history (Telegram-side data), they
just can't trigger anything new.

## Multiple Telegram clients on one account

A single Telegram account on multiple devices (phone + desktop +
tablet) shares one numeric user ID. You only need that ID once on the
whitelist; the bot accepts updates from any of the user's devices.

## Telegram groups vs. private DMs

macontrol is designed for **private 1-on-1 DMs** with the bot. Group
behavior:

- If you add the bot to a group, it ignores group messages from
  non-whitelisted members (same as DM).
- Whitelisted users sending commands in a group **do** trigger actions
  — the bot replies in the group, visible to all members.
- This is usually not what you want. Don't add the bot to groups
  unless you've thought about it.

For "shared access among multiple users", just whitelist each user's
ID and have them DM the bot directly.

## Bots, channels, anonymous admins

A few Telegram-specific edge cases:

- **Bot-as-sender** (another bot triggering yours) — Telegram's bot
  API doesn't let bots message other bots; this is a non-issue.
- **Channel posts** — channels can send to bots that are admins, with
  `from` set to the channel. Channels don't have a user ID in the
  whitelist sense, so they're rejected.
- **Anonymous group admins** ("Sign messages anonymously") — the
  sender appears as the group itself, not the admin. Rejected.

## Compromised whitelist

If someone gets onto your whitelist who shouldn't be (you typed the
wrong ID, you re-used a stale ID that was reassigned, etc.):

1. **Immediately** edit the config, remove the wrong ID, restart.
2. Check `~/Library/Logs/macontrol/macontrol.log` for what they did
   while authorized — every action is logged with the user ID.
3. Reverse anything destructive that they did:
   - Volume / brightness / DNS changes → reset manually
   - Kept-awake processes → `pkill -x caffeinate`
   - Started recordings → check `~/Library/Caches/` for orphaned
     tempfiles, but Telegram already has the upload (Telegram-side
     deletion only)
4. Treat the bot token as compromised too if you suspect they had
   shell access — see [Security → Bot token](../security/bot-token.md).

## Why not username-based?

Telegram `@username`s are mutable and reusable. If you whitelist
`@alice` and Alice deletes her account, Telegram can later reassign
`@alice` to someone else. Numeric IDs never change owner.

So macontrol's whitelist is intentionally numeric-only. There's no
`@username` form.

## Why not OAuth or PIN?

For a single-owner bot, the bot token + Telegram's authenticated
delivery is already proof of identity. Adding a separate auth layer
on top would be theater — Telegram already authenticated the user
when they signed in.

If multi-tenant access ever becomes a goal, OAuth-style sessions would
make sense. Today it doesn't.
