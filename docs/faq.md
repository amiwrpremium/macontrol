# FAQ

## General

### What does this bot actually do?

Lets you control your Mac from a Telegram chat. Volume, brightness,
power, network, screenshots, notifications, and more. See [Usage →
Categories](usage/categories/README.md) for the full list.

### Is this safe?

As safe as your bot token + your Mac's account password. The bot has
a hard whitelist on Telegram user IDs — non-whitelisted senders are
silently dropped. With a leaked token, an attacker can mess with
your bot but can't trigger any macOS action. With a compromised Mac
account, they have the same access you do regardless of macontrol.

See [Security → Threat model](security/threat-model.md) for the full
breakdown.

### Does it work on Intel Macs?

No. Apple Silicon only (M1/M2/M3/M4 and successors). The install
script and CI both refuse non-arm64. See
[Architecture → Design decisions](architecture/design-decisions.md#why-apple-silicon-only).

### Why Telegram and not Signal/iMessage/Discord?

Telegram has the best bot API of the major messengers — first-class
inline keyboards, callback queries, multi-step flows, file uploads
all via documented HTTP endpoints. Signal's bot story is rudimentary;
iMessage has no real bot API; Discord's is good but more focused on
servers than 1-on-1.

That said, the architecture (domain layer + UI layer) would port
cleanly to other platforms. PRs for a Discord variant would probably
be accepted as a sister project, not in this repo.

### Can I run it on a Linux server controlling a remote Mac?

No. macontrol runs **on the Mac it controls**. There's no protocol
for remote control of a Mac from a Linux box.

If you want that pattern, run macontrol on the Mac and trigger it
from Linux via the Telegram bot — that's basically what every
whitelisted user does.

## Getting started

### Where do I get a bot token?

DM [@BotFather](https://t.me/BotFather) on Telegram, send `/newbot`,
follow the prompts. See
[Getting started → Telegram credentials](getting-started/credentials-telegram.md).

### Where do I get my user ID?

DM [@userinfobot](https://t.me/userinfobot), send `/start`. The
number after `Id:` is your numeric Telegram user ID.

### The setup wizard says "token verification failed"

Either the token is wrong, was revoked in BotFather, or your network
can't reach `api.telegram.org`. See
[Troubleshooting → Common issues](troubleshooting/common-issues.md#token-verification-failed).

### Do I need to install brew dependencies?

Some features require them; the core works without any:

- `brightness` — for 💡 Display level read/set.
- `blueutil` — for 🔵 Bluetooth (entire category).
- `terminal-notifier` — for richer 🔔 Notify (osascript fallback if
  missing).
- `smctemp` — for °C in 🖥 System → Temperature.
- `imagesnap` — for 📸 Webcam photo.

Install all five at once:

```bash
brew install brightness blueutil terminal-notifier smctemp imagesnap
```

`macontrol doctor` reports which are missing.

## Usage

### How do I lock my screen quickly?

Send `/lock` from Telegram. No menu navigation needed.

### How do I see my Mac's status without opening the menu?

Send `/status`. One message with battery, OS info, Wi-Fi state, uptime.

### Can the bot push me alerts (e.g. battery low)?

No. macontrol is request-response, not push-driven. For "tell me when
X happens" use macOS's Shortcuts app personal automations, which has
triggers for battery level, time of day, location, etc.

### Why isn't there a button bar at the bottom of the chat?

We dropped the bottom-of-input ("reply") keyboard in v0.1.4 because
it duplicated the inline home grid. All navigation now happens via
inline keyboards attached to messages: every category dashboard has
a 🏠 Home button that returns to the inline grid, and `/menu` always
sends a fresh grid. See
[Usage → UX model](usage/ux-model.md) for the rationale.

### How do I cancel a multi-step flow?

Send `/cancel`. The active flow drops and the next message you
send is treated as a normal message.

### Why is the speedtest button missing?

You're on macOS 11 (Big Sur). `networkQuality` was added in macOS 12.
See [Reference → Version gates](reference/version-gates.md).

### Why is the Run Shortcut button missing?

You're on macOS 11 or 12. `shortcuts` CLI was added in macOS 13
(Ventura). Same reference link.

### Can I add my own shortcuts to the keyboard?

Not directly — the categories are fixed. But you can author a macOS
Shortcut and run it via **🛠 Tools → Run Shortcut…**. That's the
intentional escape hatch for "I need something macontrol doesn't
have".

### Why does the bot not respond when I add it to a group?

Because the group's user IDs aren't on your whitelist. The bot only
acts on messages from whitelisted user IDs, regardless of where
they're sent. Adding the bot to a group is rarely useful — DM it
directly.

## Configuration

### Where is the config file?

There isn't one. The bot token and whitelist live in the macOS
Keychain (written by `macontrol setup`); runtime knobs are CLI
flags. See
[Configuration → Runtime](configuration/runtime.md) and
[Configuration → File locations](configuration/file-locations.md).

### How do I add another whitelisted user?

```bash
macontrol whitelist add 987654321
brew services restart macontrol
```

See [Configuration → Whitelist](configuration/whitelist.md).

### How do I change the log level?

Edit `~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist` and
set `ProgramArguments` to include `--log-level=debug`, then
`brew services restart macontrol`. Switch back to `info` after
diagnosing.

For an ad-hoc session without touching the plist:

```bash
macontrol service stop
macontrol run --log-level=debug --log-file=
```

### Can I run multiple bot tokens against the same Mac?

Yes, but cleanest is to run each daemon under a separate macOS
user. Each user has its own login keychain, so the tokens stay
isolated, and each gets its own LaunchAgent label and log
directory.

## Permissions

### Why does macOS ask for "Screen Recording" permission?

Because macontrol is about to take a screenshot or recording. Click
**Open System Settings**, toggle `macontrol` on, restart the daemon,
re-tap the button. See [Permissions → TCC](permissions/tcc.md).

### What if I clicked Don't Allow on a permission prompt?

Open System Settings → Privacy & Security → the relevant section,
toggle `macontrol` on. macOS won't re-prompt; you have to add it
manually. Then restart the daemon.

### Why does the temperature button return "unknown"?

Either:

- `sudo powermetrics` failed because the narrow sudoers entry isn't
  installed. Install it via `macontrol setup --reconfigure`.
- `smctemp` isn't installed (only affects the °C readings, not the
  pressure level). Install with `brew install narugit/tap/smctemp`
  (it lives outside homebrew-core, in the `narugit/tap` third-party
  tap).

### Do I have to install the sudoers entry?

No, but you'll lose:

- 🌡 Temperature readings
- 📶 Wi-Fi → Info (wdutil)
- 🛠 Tools → Timezone…
- 🛠 Tools → Sync time

The setup wizard offers it. You can install or remove it later via
`macontrol setup --reconfigure`.

### Is the sudoers entry safe?

It only grants `NOPASSWD` for five specific binaries with their
specific arguments — `pmset`, `shutdown`, `wdutil info`,
`powermetrics`, `systemsetup`. Not blanket sudo. Even if the daemon
is compromised, the attacker can only invoke those five things. See
[Permissions → Sudoers](permissions/sudoers.md).

## Operations

### How do I update macontrol?

```bash
brew upgrade macontrol && brew services restart macontrol
```

See [Operations → Upgrades](operations/upgrades.md).

### How do I uninstall it cleanly?

```bash
macontrol service uninstall
brew uninstall macontrol
rm -rf ~/Library/Application\ Support/macontrol ~/Library/Logs/macontrol
sudo rm /etc/sudoers.d/macontrol     # if you installed it
```

See [Configuration → File locations → Clean uninstall](configuration/file-locations.md#clean-uninstall).

### Where do I see what the bot is doing?

```bash
tail -f ~/Library/Logs/macontrol/macontrol.log
```

Or use Console.app — it picks up `~/Library/Logs/` automatically.

### The daemon stopped running

```bash
launchctl list | grep macontrol
```

If the PID column shows `-`, it crashed and is in restart backoff.
Check the log for the panic. If it's not even loaded, run:

```bash
brew services start macontrol
# or
macontrol service start
```

## Development

### Can I add a new category / button?

Yes. See [Development → Adding a capability](development/adding-a-capability.md)
for the recipe.

### How do I run tests?

```bash
make test            # quick
make test-race       # with race detector + coverage
```

See [Development → Testing](development/testing.md).

### Why isn't my PR's CI running?

Either:

- You're not signed into GitHub on the fork.
- Your fork's Actions are disabled (forks have Actions off by default
  for non-pushed branches).

The maintainer can re-run CI on PRs from forks via "Approve and run
workflows" in the PR's Checks tab.

### Why does my PR title fail the check?

It's not a Conventional Commit. See
[Development → Conventional Commits](development/conventional-commits.md).

### Can I propose a major refactor?

Open a Discussion first to gauge interest. Major refactors PR'd
without prior discussion often get closed. Solo-maintained means the
maintainer's bandwidth is the bottleneck — please respect it.

## Misc

### Why is the version v0.x?

The project hasn't been smoke-tested on a real Mac yet at the time
of this writing. v0.x signals "early; behavior may change". Once
the project is stable and has some users, it'll graduate to v1.0.

See [Architecture → Design decisions](architecture/design-decisions.md#why-initial-version-is-pinned-to-v010)
and [Development → Releasing](development/releasing.md).

### Is there a Discord/Slack community?

No. Communication happens on GitHub: Issues for bugs and feature
requests, Discussions for questions and ideas. The maintainer reads
both.

### Can I contribute money?

If a GitHub Sponsors button shows on the repo, sure. Otherwise, no
payment infrastructure. PRs and bug reports are the most useful
contributions.

### Can I fork it?

It's MIT-licensed. Do whatever — the only thing the license forbids
is removing the copyright notice.
