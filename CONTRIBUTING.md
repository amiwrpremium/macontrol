# Contributing to macontrol

Thanks for your interest. macontrol is a small Go project, so the process is
small too.

## Workflow

1. **Fork** the repo and create a branch off `master`.
2. **Commit** using [Conventional Commits](https://www.conventionalcommits.org). A `.gitmessage` template is provided — enable it with:
   ```bash
   git config commit.template .gitmessage
   ```
3. **Run checks** locally before pushing:
   ```bash
   make lint test
   ```
4. **Open a PR** against `master`. The PR **title** must be a valid
   Conventional Commit — a GitHub Action rejects non-conforming titles. We
   squash-merge, so only the PR title lands in the history.

## Commit conventions

- Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`,
  `chore`, `revert`.
- Scopes mirror the project tree: `sound`, `display`, `power`, `wifi`, `bt`,
  `battery`, `system`, `media`, `notify`, `tools`, `bot`, `flows`,
  `keyboards`, `callbacks`, `runner`, `config`, `ci`.
- Breaking changes: add `!` after the scope (`feat(bot)!: …`) and a
  `BREAKING CHANGE:` footer. release-please will bump the major version.
- Keep subjects in imperative mood, lower-case, under 72 chars, no period.

## Adding a new capability

Say you want to add `/spotify`:

1. `internal/domain/spotify/spotify.go` — pure control, no Telegram imports.
2. `internal/telegram/keyboards/spotify.go` — inline keyboard builder.
3. `internal/telegram/callbacks/spo.go` — callback_data dispatch.
4. `internal/telegram/handlers/spo.go` — glue between domain + keyboard.
5. Register in `internal/telegram/keyboards/home.go` (home button) and
   `internal/telegram/callbacks/registry.go` (route).
6. Write a domain test using the `runner.Fake` helper and a
   keyboard-layout test.

## Development setup

### From Linux (most contributors)

The domain layer is mocked via the `runner.Runner` interface and the
Telegram layer is tested against an `httptest`-backed bot
(`internal/telegram/telegramtest`). Neither needs a real bot token, a
real Keychain, or macOS itself. The full unit-test loop runs anywhere
Go runs:

```bash
make lint test                 # golangci-lint + go test -race
make build                     # cross-compile darwin/arm64 sanity check
```

You can implement and ship most features without a Mac. The pieces
that genuinely need one are:

- TCC-gated subprocesses (screencapture, imagesnap, osascript)
- LaunchAgent installation
- Real Keychain reads / writes
- End-to-end smoke against a Telegram bot

If your change touches one of those, hand off to a Mac contributor or
mark the PR `needs-mac-test`.

### From a Mac (iterating on local changes)

Run your locally-built binary against your real Keychain entry. The
loop:

```bash
# One-time: write a dev token + whitelist to the Keychain.
macontrol setup

# Per-iteration:
brew services stop macontrol         # or: macontrol service stop
make build                           # → dist/macontrol
./dist/macontrol run --log-file= --log-level=debug
# … test in Telegram … Ctrl-C when done …
brew services start macontrol        # back to the production daemon
```

`--log-file=` (empty) sends logs to stderr so you see them live in
the terminal. `--log-level=debug` adds per-update routing, callback
parses, and subprocess command lines.

#### Keychain ACL prompts

The Keychain ACL on macontrol's entries is **binary-path-based**
until code signing lands. Your `dist/macontrol` is a different path
than `/opt/homebrew/bin/macontrol`, so the first run triggers a
macOS prompt asking to read the Keychain entry. Click **Always
Allow** once and the prompt won't recur for that path.

To skip the prompt entirely:

```bash
./dist/macontrol token reauth        # re-issues the ACL for this path
```

**Don't use `go run` for iterative dev.** It compiles to a fresh
random temp path each invocation, so the ACL prompt fires every
time. Build to a stable path (`dist/macontrol`) and run that.

#### Testing with a non-production token

Two options:

1. Swap the token in-place:
   ```bash
   macontrol token set                # paste dev token
   # … iterate …
   macontrol token set                # paste production token back
   ```
2. Run under a separate macOS user — each user has its own login
   keychain, so the two tokens stay isolated:
   ```bash
   sudo -u devbot ./dist/macontrol setup
   sudo -u devbot ./dist/macontrol run --log-file=
   ```

#### Don't run two daemons against one token

Telegram delivers each update to whichever client is polling at the
moment. If your local `dist/macontrol run` and the brew-installed
daemon both poll the same token, updates are randomly split between
them and the bot looks flaky. Always `brew services stop macontrol`
before running locally.

## Releases

Maintainers don't tag manually. `release-please` opens a release PR whenever
`master` accumulates release-worthy commits; merging that PR tags the
version, and GoReleaser takes over from there (tarball + Homebrew formula).
