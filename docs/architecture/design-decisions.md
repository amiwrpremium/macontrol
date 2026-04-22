# Design decisions

The "why" behind every choice that's likely to surprise a reader. If
you ever ask "why don't they just do X" while reading the code, the
answer is probably here.

## Why Go

- **Single static binary** — no runtime, no `pip install`, no `node
  modules`. Copy one file to a Mac, you're done. The brew tap publishes
  a 7 MB Mach-O.
- **Cross-compile from anywhere** — `GOOS=darwin GOARCH=arm64 go build`
  produces the Mac binary on Linux. CI doesn't need a Mac for the
  build job.
- **Fast startup** — every Telegram update spawns a subprocess.
  Python's ~100 ms cold start would compound on every `osascript`,
  `pmset`, `networksetup` call. Go's ~5 ms keeps the bot snappy.
- **Standard concurrency** — goroutines + `context.Context` for
  per-request timeouts. The flow registry's TTL janitor is one
  goroutine + one ticker.
- **Strong stdlib** — `os/exec`, `log/slog`, `encoding/json`,
  `net/http` cover almost everything. Minimal dep tree.

Trade-off: refactoring is more verbose than Python. Acceptable for a
project this size.

## Why Apple Silicon only

Three reasons stack up:

1. **Intel `powermetrics` reports different fields** than ASi. Half
   the parsers would need split paths.
2. **`smctemp` and the SMC sampler** behave differently on Intel.
   Maintaining both is overhead for a feature that's already best-effort.
3. **Apple stopped selling Intel Macs in 2023**. Going forward, all
   new Macs are ASi. Targeting both is paying maintenance cost for
   a shrinking install base.

The install script and CI both refuse to run on `arm64` non-Darwin or
on Intel Darwin. It's not "we haven't tested it on Intel" — it's "we
intentionally don't ship there".

## Why named commands only — no `/sh` escape hatch

A `/sh <cmd>` would be trivial to add. The bot has the user's shell
privileges. A whitelisted user could `/sh rm -rf ~/`.

Two scenarios where this becomes a real problem:

- **Compromised whitelist** — if you accidentally add the wrong user
  ID, they get arbitrary shell. With named commands, the worst they
  can do is what's enumerated in [Usage → Categories](../usage/categories/README.md).
- **Compromised bot token** — if the token leaks (without your user
  ID being on the list, an attacker still can't act, but with
  per-user whitelists removed in the future this becomes a thing).

The Shortcuts runner (`tls:shortcut`) is the deliberate compromise:
**you** author the shortcut, naming it explicitly, and it's runnable
by name — no arbitrary shell, but full power if you set it up.

## Why no reply keyboard (inline-only navigation)

Telegram offers two keyboard surfaces: **reply keyboards** (the bar
that appears below the input field) and **inline keyboards**
(buttons attached to a specific message). Early macontrol had both
— the home grid existed as a reply keyboard for discovery *and* as
an inline grid on `/status` for in-flight navigation. That
redundancy was dropped in v0.1.4.

Reasons the reply keyboard went away:

- **Chat pollution.** Tapping a reply-keyboard button makes
  Telegram echo the label as a user-sent message. After a few
  taps, chat history is a wall of `🔊 Sound / 💡 Display /
  🔊 Sound` bubbles with no context — noise, not signal.
- **Redundant.** Every category label on the reply keyboard also
  exists as an inline button on the home grid. Two paths to the
  same place confuses users without adding capability.
- **Eats input area or collapses awkwardly.** Persistent keyboards
  consume ~40% of phone screen; one-shot keyboards disappear after
  first tap, requiring `/menu` to resummon.
- **Inline keyboards edit in place** — the Sound dashboard mutates
  when you tap `+5` without spawning a new message. Reply
  keyboards can't do that.

The inline home grid is reachable three ways: send `/menu`, send
`/status`, or tap the 🏠 Home button on any leaf dashboard (which
edits the current message back to the home grid). That's enough
navigation surface without a second redundant one.

Slash-command discoverability comes from Telegram's
`/setcommands` menu (configured via `@BotFather`), which shows
the `/` button next to the input field.

## Why inline keyboards edit-in-place

Two alternative designs:

- **Send a new message per tap** — simple to implement, but the chat
  fills with dozens of "Volume: 60% / 65% / 70%" messages. Useless.
- **Always-up status message that updates** — would need a long-running
  goroutine per chat, polling state. Heavyweight for a bot that should
  be idle when nobody's looking.

Edit-in-place gives the live-dashboard feel without any background
polling. Each tap = one Telegram API call, one subprocess.

## Why the 64-byte callback protocol with a shortmap

Telegram caps `callback_data` at **64 bytes per button**. That's
enough for `snd:up:5` (8 bytes) but not for a Bluetooth MAC
(`bt:conn:00-11-22-33-44-55-66` is 28 bytes — fits, but a longer
SSID (`wif:join:My Office's Guest Wi-Fi 5GHz Channel`) overflows.

Two ways to handle this:

- **Encode the long arg in the message text and pick it up via
  another mechanism** — fragile, breaks the "callback_data is the
  source of truth" model.
- **Store the long arg server-side, put a short id in callback_data**
  — what we do.

`callbacks.ShortMap` is an in-memory map keyed by 10-char base32 ids
with a 15-minute TTL. The button carries `bt:conn:<short-id>`; the
handler resolves the short-id back to the full MAC.

15-minute TTL because dashboards aren't meant to be left open for an
hour. If you tap a stale button, you get "session expired; refresh
the device list."

## Why the flow registry has a 5-minute TTL

User starts a flow ("set exact volume"), gets distracted, never
sends the number. Without a timeout, the flow sits there forever
consuming a map entry. Worse, a year later you tap "Set exact value"
and the previous flow's stale state interferes.

5 minutes is "long enough for normal use, short enough that distraction
doesn't pile up state". A janitor goroutine sweeps every 2.5 minutes.

## Why no built-in DDC support for external monitors

External monitor brightness via DDC/CI is a deep rabbit hole:

- `ddcctl` works on most monitors but not all (depends on the monitor's
  DDC implementation).
- `betterdisplaycli` works for more monitors but is a paid app.
- macOS's system-level brightness slider already controls the
  built-in display via Apple's protocol; DDC is a separate, racier
  channel.

Adding it to the dashboard would require: detecting attached monitors,
choosing one in the UI, handling per-monitor differences, supporting
multiple DDC tools as alternatives. Big surface area for what is
effectively a power-user feature.

The current scope is **built-in display only**, which works without any
external dependencies. Open an issue if you want external monitor
control — it'd land as an opt-in feature, not the default path.

## Why no Wi-Fi scanning

Apple removed `airport -s` (the private CLI for scanning) in macOS
14.4 and hasn't shipped a public replacement. `wdutil info` doesn't
scan. `networksetup` doesn't scan.

Options:

- **Parse `system_profiler SPAirPortDataType`** — slow (1–2 seconds),
  output format isn't documented as stable.
- **Ship a Swift helper using `CoreWLAN.CWWiFiClient.scanForNetworks`**
  — adds a Swift toolchain dep to the build, and CoreWLAN itself has
  spotty TCC behavior.

Workaround for users: tap the macOS Wi-Fi menu manually to see
networks, then `🔗 Join network…` with the SSID you want.

If a real official CLI ships in a future macOS, scanning gets added
as a button.

## Why no fan control or per-sensor thermal

Apple Silicon's fan and thermal subsystem is opaque to user-space:

- `pmset` doesn't expose fan RPM.
- The SMC sampler that worked on Intel doesn't exist on ASi.
- `powermetrics` reports a coarse "thermal pressure" enum
  (Nominal/Moderate/Heavy/Trapping/Sleeping), not °C.
- `smctemp` (brew) gives approximate °C but values can oscillate;
  Apple doesn't document the SMC keys it reads.

So the System → Temperature dashboard shows what we can show:
pressure level (always available with sudo) and °C readings (when
`smctemp` is installed, with a "may be unstable" note).

Fan control is impossible from user-space on ASi. Not supported.

## Why three SDK clients aren't a thing

(Inherited from the kucoin-monitor reference repo: that bot used
three KuCoin SDK clients for blast-radius isolation. macontrol
doesn't have an analog because it's calling 30+ macOS CLIs, not one
SDK with credential scopes.)

## Why prefer `terminal-notifier` over osascript

`osascript display notification`:

- Always available, no brew dep.
- No control over sound, no action buttons, no `-group` for dedup.

`terminal-notifier`:

- Brew install.
- Supports `-sound default`, `-group macontrol` (dedup so spam doesn't
  pile up), `-open URL` (clickable), `-execute CMD` (runs a shell
  command on click).

We try `terminal-notifier` first; if it's not on `$PATH`, fall back to
`osascript`. The bot reports which transport it used so you know.

## Why `runner.Runner` is an interface

Could be a concrete type with `os/exec` baked in. Making it an
interface enables `runner.Fake` for tests:

```go
type Runner interface {
    Exec(ctx context.Context, name string, args ...string) ([]byte, error)
    Sudo(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

Every domain test uses `runner.NewFake().On("expected cmd", "stdout",
nil)` — no real subprocess invocations, no platform requirements. Tests
run on Linux CI.

The cost is one extra interface; the payoff is 90%+ unit-test coverage
on the domain layer without any macOS calls.

## Why `telegramtest` instead of mocking the bot library

Three options for testing handlers:

1. **Mock the bot library** — define our own minimal `Bot` interface,
   inject a stub. Big refactor; every handler signature changes.
2. **Use the library's documented test hooks** — `tgbot.WithServerURL`
   + `tgbot.WithSkipGetMe`. We point a real `*Bot` at an in-process
   `httptest.Server` that records every API call. Zero production-code
   changes.
3. **Don't test handlers** — leave them as integration-only. Fragile.

Option 2 is what `internal/telegram/telegramtest/server.go` does. The
`Recorder` exposes `ByMethod("sendMessage")` etc. for assertions.

This means our tests exercise the real Telegram client code paths
(URL construction, multipart payload building, JSON unmarshalling) —
not just our wrapper. Higher fidelity.

## Why `release-please` + GoReleaser instead of one tool

Each does one job well:

- `release-please` — reads conventional-commit messages, computes
  the next version, opens a PR with the version bump + CHANGELOG
  update. No artifacts, no upload.
- `GoReleaser` — given a tag, builds the binary, packages tarballs,
  computes checksums, creates the GitHub Release, updates the
  Homebrew tap. No version computation.

Combining them: you merge the release-please PR (which tags), and
that triggers GoReleaser. Each tool stays simple.

## Why initial-version is pinned to v0.1.0

`release-please` defaults to v1.0.0 for Go projects on the first
release (Go's convention is that <1.0 is unstable, so it skips
straight to 1.0). For a brand-new bot that hasn't been smoke-tested
on a real Mac yet, that's overclaim.

`packages["."].initial-version: "0.1.0"` in
`release-please-config.json` overrides this to start at v0.1.0. Once
the project is stable enough to commit to API guarantees, we can
manually tag v1.0.0 and remove the override.

## Why no translations / i18n

Single owner, English-speaking. Adding a translation framework is a
4-week project and serves zero current users. If a non-English user
files an issue, we'll cross that bridge.

## Why the docs/ structure is what it is

Modeled after `amiwrpremium/kucoin-telegram-monitor`'s docs/, which is
in turn modeled after a pattern from cloud-native projects (architecture,
configuration, deployment, operations, reference, security,
troubleshooting, development). Every directory has a `README.md` so
GitHub renders the directory listing nicely without forcing the reader
to open a file first.

The "why" prose follows the kucoin pattern: **what → why → how →
example → edge cases**. Skimming this doc itself is the worked example
of that style.
