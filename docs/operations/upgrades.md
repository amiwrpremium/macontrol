# Upgrades

How to move to a newer macontrol release without losing config or
breaking the running daemon.

## Homebrew

```bash
brew update
brew upgrade macontrol
brew services restart macontrol
```

That's the whole sequence. Breakdown:

1. **`brew update`** refreshes Homebrew's view of taps. Pulls the
   latest formula from `amiwrpremium/homebrew-tap`.
2. **`brew upgrade macontrol`** downloads the new release tarball,
   verifies the SHA-256 against the formula's expected hash,
   atomically replaces the binary at `/opt/homebrew/bin/macontrol`.
   The currently-running daemon is **not** affected — the on-disk
   binary changed but the running process is still the old version.
3. **`brew services restart macontrol`** stops and re-starts the
   daemon, which now loads the new binary. Brief (1–2 second) gap in
   responsiveness during the restart.

### Pinning a version

If you want to skip an upgrade:

```bash
brew pin macontrol               # pin to current version
# …time passes, brew upgrade is run, macontrol stays put…
brew unpin macontrol             # release the pin
```

To install a specific version:

```bash
brew install amiwrpremium/tap/macontrol@v0.2.0
```

(Requires the tap to publish versioned formulae, which it does for
any release.)

## Manual install

```bash
# Stop the daemon
macontrol service stop

# Download + replace the binary (uses the same install script)
curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | sh

# Restart
macontrol service start
```

The install script always downloads the **latest** release. If you
want a specific version:

```bash
TAG=v0.2.0
curl -fsSL https://github.com/amiwrpremium/macontrol/releases/download/${TAG}/macontrol_${TAG#v}_darwin_arm64.tar.gz \
  | tar -xz -C /tmp
sudo install -m 0755 /tmp/macontrol /usr/local/bin/macontrol
macontrol service start
```

Verify the SHA-256 manually (the install script does this for you):

```bash
shasum -a 256 /tmp/macontrol_${TAG#v}_darwin_arm64.tar.gz
# compare against checksums.txt at:
# https://github.com/amiwrpremium/macontrol/releases/download/${TAG}/checksums.txt
```

## Built from source

```bash
cd ~/path/to/macontrol-clone
git fetch && git checkout v0.2.0
make build
sudo install -m 0755 dist/macontrol $(which macontrol)
macontrol service stop && macontrol service start
```

If you've added local changes you don't want to lose, use `git stash`
or a feature branch.

## What changes between versions

Every release has a corresponding GitHub Release with a CHANGELOG
section. Look at it before upgrading:

<https://github.com/amiwrpremium/macontrol/releases>

The CHANGELOG categorizes changes by type (Features, Bug Fixes,
Performance, Refactors). Pre-1.0 releases (v0.x.y) may include
breaking changes within minor bumps; from v1.0.0 onward the project
follows strict SemVer.

### Breaking changes are flagged

In the CHANGELOG, breaking changes are marked with `!`:

```text
### Features

- feat(bot)!: switch from numeric user IDs to OAuth tokens for whitelist
  BREAKING CHANGE: numeric user-ID whitelist removed. Re-run `macontrol setup`.
```

If you see one, read the `BREAKING CHANGE:` footer for the migration
steps. The CHANGELOG and the GitHub Release notes both surface them.

## Config changes

Secrets stay in the Keychain across upgrades — the entries are
written once by `macontrol setup` and the schema (one entry for the
token, one for the comma-separated whitelist) is stable. Upgrades do
not touch your existing entries.

Runtime flags (`--log-level`, `--log-file`) are read from the
LaunchAgent plist. If a release adds a new flag, the default is
sensible enough that you don't need to update the plist; the
CHANGELOG calls out any flag whose default would change behavior.

## Keychain ACL re-prompts after a path change

The Keychain ACL on macontrol's bot-token entry is binary-path-based
(until code signing lands in v1.x). After an upgrade that changes the
binary's path — common when switching between brew install and manual
install, or after a brew bottle relocation across macOS major versions
— macOS will re-prompt for Keychain access on the daemon's first read
of the new binary. The bot will appear unresponsive while the prompt
sits unaddressed.

Two recovery paths:

### Click "Always Allow" on the prompt

If you're at the Mac when the daemon starts and macOS shows the
prompt, click **Always Allow**. From then on, reads are silent.

### Re-grant ACL non-interactively

```bash
macontrol token reauth
brew services restart macontrol
```

This reads the existing Keychain entry and re-issues it with the
current binary path in the ACL. Daemon's next start reads silently.

## Verifying the upgrade worked

```bash
macontrol --version
# Expected: macontrol v0.2.0 (xyz1234, 2026-05-15)
```

And:

```bash
brew services info macontrol
# Status should be `started` with a recent PID
```

And from Telegram:

```text
/status
```

You should see the boot ping (re-issued on every restart since the
daemon initializes whitelist messaging at startup).

## Rolling back

```bash
brew install amiwrpremium/tap/macontrol@v0.1.0   # the previous version
brew services restart macontrol
```

Or via curl:

```bash
TAG=v0.1.0
curl -fsSL https://github.com/amiwrpremium/macontrol/releases/download/${TAG}/macontrol_${TAG#v}_darwin_arm64.tar.gz \
  | tar -xz -O macontrol > /tmp/macontrol-rollback
sudo install -m 0755 /tmp/macontrol-rollback /usr/local/bin/macontrol
macontrol service stop && macontrol service start
```

Keychain entries written by a newer setup wizard are always readable
by an older binary (the format hasn't changed since v0.1.0). If a
breaking-change boundary did change the entry shape, the rollback
steps would be in the CHANGELOG.

## Subscribing to releases

GitHub:

1. Open <https://github.com/amiwrpremium/macontrol>
2. Click the **Watch** dropdown (top-right) → **Custom** → **Releases**.

You'll get an email each time a new release is tagged.

## Upgrading brew dependencies

The optional brew formulae (`brightness`, `blueutil`, `terminal-notifier`,
`smctemp`, `imagesnap`) get their own updates via `brew upgrade`:

```bash
brew upgrade brightness blueutil terminal-notifier smctemp imagesnap
```

These don't require a macontrol restart — the daemon shells out fresh
on every invocation.

## Rare: upgrading sudoers entry

If a release adds a new sudoers-needing binary, the entry installed by
older `macontrol setup` won't include it. Re-run:

```bash
macontrol setup --reconfigure
# answer 'y' to the sudoers prompt to re-install
```

The wizard validates the new content with `visudo -cf` before installing,
so you can't break sudo this way.
