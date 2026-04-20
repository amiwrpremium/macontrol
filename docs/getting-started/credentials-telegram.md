# Telegram credentials

You need two things before running `macontrol setup`:

1. A **bot token** — identifies your bot to Telegram. Created by DM'ing
   @BotFather.
2. Your own **Telegram user ID** — the numeric ID, not your @username.
   Used as the first entry on the bot's whitelist. Obtained from
   @userinfobot.

Both are free and take under two minutes.

## 1. Create the bot

### DM @BotFather

1. Open Telegram.
2. Search for [@BotFather](https://t.me/BotFather) and start a chat.
3. Send `/newbot`.

### Pick a display name

BotFather asks for a **display name** first. This is what shows above
messages from your bot. Something like:

```text
macontrol (amiwrpremium's Mac)
```

### Pick a username

Then it asks for a **username**. This must:

- End in `bot` (case-insensitive — `Bot`, `_BOT` all work).
- Be globally unique across Telegram.
- Be 5–32 characters.

Example:

```text
amiwrpremium_macontrol_bot
```

### Copy the token

BotFather replies with something like:

```text
Done! Congratulations on your new bot. You will find it at t.me/amiwrpremium_macontrol_bot.
You can now add a description, about section and profile picture for your bot, see
/help for a list of commands. By the way, when you've finished creating your cool bot,
ping our Bot Support if you want a better username for it.

Use this token to access the HTTP API:
123456789:AAE-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456

Keep your token secure and store it safely, it can be used by anyone to control your bot.
```

The long string after "Use this token to access the HTTP API:" is your
`TELEGRAM_BOT_TOKEN`. Copy it somewhere temporary (paste buffer or a
scratch note).

**Treat this token like a password.** Anyone with it can impersonate
your bot and, if your user ID isn't whitelisted, also send messages *as
you* in Telegram groups where your bot is present. See
[Security → Bot token](../security/bot-token.md).

### Optional BotFather tweaks

Nice-to-have polish, none required:

- `/setdescription` — one-line text shown on the bot's profile.
  Suggested: *"Remote control for amiwrpremium's Mac. Owner-only."*
- `/setuserpic` — profile avatar.
- `/setprivacy` → **Enable** — the bot doesn't read other people's
  messages in groups. Doesn't affect outgoing messages. macontrol is
  designed for private 1-on-1 use so this is recommended.
- `/setcommands` — paste the list of slash commands so Telegram shows
  them as a menu in the chat's attachment area:
  ```text
  start - Show the home keyboard
  menu - Show the home keyboard
  status - Dashboard snapshot
  help - Slash-command help
  cancel - Cancel an active flow
  lock - Lock the screen
  screenshot - Full-screen silent screenshot
  ```

## 2. Get your Telegram user ID

macontrol's whitelist is numeric-ID-based, not `@username`-based.
Usernames can be changed or deleted; numeric IDs are permanent.

### DM @userinfobot

1. Open Telegram.
2. Search for [@userinfobot](https://t.me/userinfobot) and start a
   chat.
3. Send `/start`.

It replies instantly with something like:

```text
👤 Deleted Account
🆔 Id: 123456789
📧 Username: none
💬 Language: en
```

The number after `Id:` is your `ALLOWED_USER_IDS` entry. It's a 9–10
digit number.

### Multiple users

If you want to allow more than one person (e.g. your phone and your
laptop's Telegram Desktop on the same account — they share the same
user ID — or a family member on a different account), collect each
user's ID the same way and comma-separate them later:

```dotenv
ALLOWED_USER_IDS=123456789,987654321,555444333
```

## 3. Send a message to your own bot

Before `macontrol setup` and the daemon will work, **you have to message
your bot first at least once**. Until you do, Telegram refuses to let
any bot initiate a conversation with you.

1. Open the bot's DM: `https://t.me/<your-bot-username>` or tap the
   link BotFather gave you.
2. Tap **Start** or send any message (e.g. `hello`).

You'll see no reply yet — the daemon isn't running. That's fine.

## 4. Smoke-test the token

Optional but recommended — confirms the token is valid and the bot is
alive before you wire up the daemon:

```bash
TOKEN='123456789:AAE-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456'
curl -s "https://api.telegram.org/bot${TOKEN}/getMe" | python3 -m json.tool
```

A healthy bot returns:

```json
{
    "ok": true,
    "result": {
        "id": 123456789,
        "is_bot": true,
        "first_name": "macontrol (amiwrpremium's Mac)",
        "username": "amiwrpremium_macontrol_bot",
        "can_join_groups": true,
        "can_read_all_group_messages": false,
        "supports_inline_queries": false
    }
}
```

If `"ok": false`, the `description` field tells you why. Common values:

- `Unauthorized` — token is wrong or was revoked in BotFather (`/revoke`).
- `Not Found` — you truncated or mistyped the token.

Also test sending yourself a message:

```bash
USER_ID=123456789
curl -s "https://api.telegram.org/bot${TOKEN}/sendMessage" \
  -d "chat_id=${USER_ID}" \
  -d "text=smoke-test from curl" \
  | python3 -m json.tool
```

If you see `"ok": true` and a message arrives in your DM with your bot,
everything is wired up.

If you see:

- `"chat not found"` — you haven't messaged the bot yet (step 3 above).
- `"Forbidden: bot was blocked by the user"` — you've blocked the bot.
  Unblock via its profile → three-dot menu → Unblock.

## Next step

You now have:

- `TELEGRAM_BOT_TOKEN` — the long `123456789:AAE-…` string.
- `ALLOWED_USER_IDS` — your numeric user ID (comma-separated list if
  more than one).

→ [Quickstart](quickstart.md) — run `macontrol setup` and paste these.
