# Telegram errors

HTTP codes and `description` strings from `api.telegram.org`. Each
section maps the error to a likely cause and a fix.

## 401 Unauthorized

```text
ERROR  bot exited  err="bot.New: Unauthorized"
```

**Cause**: token is wrong or revoked.

**Fix**:

- Verify the token by calling `getMe` directly:
  ```bash
  curl -s "https://api.telegram.org/bot<token>/getMe"
  ```
  If `{"ok":false,"error_code":401,...}`, the token is rejected.
- Get a fresh token from BotFather: DM @BotFather → `/token` → pick
  the bot.
- If the token in your config is correct but Telegram says it's
  unauthorized, the bot was revoked (by you or by Telegram). Create a
  new bot with `/newbot`.

## 403 Forbidden — bot was blocked by the user

```text
WARN  msg="boot ping failed"  uid=123456789  err="Forbidden: bot was blocked by the user"
```

**Cause**: that whitelisted user blocked the bot in Telegram.

**Fix**:

- Open the bot's profile in Telegram → tap **Unblock**.
- Or remove that user from `ALLOWED_USER_IDS` if they shouldn't have
  access anyway.

The boot ping for that user is silently dropped; the bot itself stays
running.

## 403 Forbidden — bot was kicked from the group/channel

If you added the bot to a group and someone removed it, the next
attempt to message that group fails. macontrol doesn't send to groups
directly (only DMs to whitelisted user IDs), so this shouldn't appear
in normal use.

## 404 Not Found — chat not found

```text
WARN  msg="boot ping failed"  uid=123456789  err="chat not found"
```

**Cause**: the user ID exists on Telegram but they've never messaged
your bot. Telegram doesn't let bots initiate conversations.

**Fix**: have that user open the bot's link
(`https://t.me/<your-bot-username>`) and send any message (e.g. `/start`).
After that, the bot can DM them.

## 409 Conflict — terminated by other getUpdates request

```text
ERROR  telegram transport error  err="Conflict: terminated by other getUpdates request"
```

**Cause**: another process is also long-polling with the same token.
Telegram only allows one long-poller per bot token.

**Fix**: find the other process and stop it.

```bash
ps aux | grep macontrol
launchctl list | grep macontrol
```

Common scenarios:

- You ran `macontrol run` in a terminal and forgot to stop it before
  `brew services start`.
- You have `macontrol run` running on another Mac with the same
  token.
- A debug session via `go run` is still alive.

## 429 Too Many Requests

```text
WARN  msg="callback dispatch"  err="429 Too Many Requests: retry after 30"
```

**Cause**: Telegram rate-limited your bot. Limits depend on the
endpoint:

- 30 messages per second per chat.
- 20 different chats per second.
- 1 message per second per group/channel.

macontrol doesn't typically hit these — a human tapping buttons
generates ~1 request per second max. Hitting the limit usually means:

- A button is being held down on the client (auto-repeat), generating
  many callbacks.
- A flow has a bug causing it to send replies in a tight loop.
- A Shortcut you're running is sending many notifications.

**Fix**:

- Stop holding the button; tap once and wait.
- The library auto-retries after the indicated delay, so transient
  rate limits self-resolve.

## 500-level — Telegram server error

```text
ERROR  telegram transport error  err="Internal Server Error"
```

**Cause**: Telegram is having problems.

**Fix**: nothing on your side. The library retries with exponential
backoff. Check <https://downdetector.com/status/telegram/> if it
persists more than a few minutes.

## Network errors

### "context deadline exceeded" on long-poll

```text
ERROR  telegram transport error  err="context deadline exceeded"
```

**Cause**: Long-poll request hit its timeout (typically 50 s) without
receiving an update. Normal, not an error — the library re-issues
the request immediately.

If you see thousands of these, check your network — they should be
infrequent during normal use (one every minute or two during quiet
periods).

### "connection refused" / "no such host"

```text
ERROR  telegram transport error  err="dial tcp: lookup api.telegram.org: no such host"
```

**Cause**: DNS resolution failed.

**Fix**:

- Check `cat /etc/resolv.conf` (rare on macOS — usually DNS is
  managed by `scutil --dns`).
- Try `dig api.telegram.org` to verify resolution works.
- If DNS works but TCP doesn't (e.g. corporate firewall blocking
  Telegram), there's nothing macontrol can do.

### "TLS handshake error"

Telegram only accepts TLS. If you're behind a MITM proxy that's
intercepting HTTPS without proper certs, the handshake fails.

**Fix**: trust the proxy's CA, or use a different network.

## Edit / send message failures

### "Bad Request: message is not modified"

```text
WARN  msg="callback dispatch"  err="Bad Request: message is not modified"
```

**Cause**: macontrol called `editMessageText` with the exact same
text + markup that's already on the message. Happens when you tap
**Refresh** but nothing changed.

This is harmless. The library logs it as a warning but the user sees
no error.

### "Bad Request: message can't be edited"

```text
WARN  err="Bad Request: message can't be edited"
```

**Cause**: trying to edit a message older than 48 hours. Telegram
blocks edits to old messages.

**Fix**: scroll to the latest dashboard message and use that, or send
`/menu` to get a fresh one.

### "Bad Request: query is too old"

```text
WARN  err="Bad Request: query is too old"
```

**Cause**: callback queries expire after a few minutes. Tapping a
button on a very old keyboard fails.

**Fix**: tap **🏠 Home** to get a fresh keyboard, or send `/menu`.

## File upload failures

### "Bad Request: file too large"

```text
WARN  err="Bad Request: file is too big"
```

Telegram's bot upload limits:

- 50 MB for photos and documents.
- 50 MB for videos.

**Cause**: a screen recording exceeded 50 MB.

**Fix**: record shorter clips. Practical limit is 5–8 seconds of
1080p screen recording before exceeding 50 MB.

### "Bad Request: PHOTO_INVALID_DIMENSIONS"

```text
WARN  err="Bad Request: PHOTO_INVALID_DIMENSIONS"
```

**Cause**: a screenshot's dimensions are too large for Telegram's
photo handling. Telegram caps photos at 10000×10000 pixels.

**Fix**: shouldn't happen on normal Macs; even an Ultra-wide 6K
display is well under that. If you see it, file a bug.

## How macontrol surfaces these

Most Telegram errors are **logged** but **not user-visible** — the
user just sees their action not respond, or no toast appears. For
errors that originate during a callback handler, the dashboard
typically edits to show `⚠ <error>`. For errors during
boot-ping or background sends, only the log captures them.

If you need to debug what's happening:

```bash
LOG_LEVEL=debug brew services restart macontrol
tail -f ~/Library/Logs/macontrol/macontrol.log
```

Then perform the failing action and read the verbose log entries.
