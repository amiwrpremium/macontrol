# Brew commands

Every Homebrew command relevant to running and managing macontrol.
Lookup-style: one or two sentences per command, grouped by purpose.

There is no `brew macontrol …` namespace — Homebrew doesn't support
per-formula custom subcommands. Anything that looks like
`macontrol <verb>` is a subcommand of the macontrol binary itself,
not of brew. See [CLI](cli.md) for those.

## Install / upgrade / remove

| Command | What it does |
|---|---|
| `brew install amiwrpremium/tap/macontrol` | First-time install. Auto-taps `amiwrpremium/homebrew-tap` if not already tapped. Pulls the five companion formulae (`brightness`, `blueutil`, `smctemp`, `imagesnap`, `terminal-notifier`) as hard deps. |
| `brew install amiwrpremium/tap/macontrol@v0.1.0` | Pin to a specific tagged version. Useful for rollback or reproducing a bug on a known release. |
| `brew upgrade macontrol` | Update to the latest released version. Replaces the binary atomically; the running daemon keeps the old binary in memory until restarted. Pair with `brew services restart macontrol`. |
| `brew reinstall macontrol` | Re-download and re-install the current version. Useful when the binary is corrupted, the install was interrupted, or you want to re-run the formula's install steps without bumping versions. |
| `brew pin macontrol` | Freeze the current version. `brew upgrade` skips it until unpinned. |
| `brew unpin macontrol` | Release the pin. The next `brew upgrade` will pick it up. |
| `brew uninstall macontrol` | Remove the binary. **Keychain entries survive** — wipe them explicitly with `macontrol token clear` and `macontrol whitelist clear` before uninstalling, or `security delete-generic-password` after. |

## Daemon lifecycle (`brew services`)

The Homebrew formula declares a `service` block, so launchd
management goes through `brew services` instead of raw `launchctl`.

| Command | What it does |
|---|---|
| `brew services start macontrol` | Bootstrap the LaunchAgent and start the daemon now. Persists across reboots. |
| `brew services stop macontrol` | Stop the daemon and bootout the LaunchAgent. Won't restart at next login until `start`ed again. |
| `brew services restart macontrol` | Stop + start. Use this after `brew upgrade` so the new binary actually runs. |
| `brew services info macontrol` | Print PID, status (`started` / `stopped` / `error`), and the plist path for macontrol. |
| `brew services list` | Status of every brew-managed service on this Mac, including macontrol. Quick way to see if multiple formulae are competing. |
| `brew services kill macontrol` | Send SIGKILL. Last resort when `stop` doesn't respond. The LaunchAgent's `KeepAlive=true` will cause launchd to restart it within a few seconds — `stop` first if you actually want it dead. |

For the underlying narrative — when to restart, what `KeepAlive`
does, sleep/wake behaviour — see
[Operations → Running](../operations/running.md).

## Inspection

| Command | What it does |
|---|---|
| `brew info macontrol` | Version, install state, dependencies, install path (`/opt/homebrew/Cellar/macontrol/<ver>/bin/macontrol`), formula caveats. The single most useful inspection command. |
| `brew deps macontrol` | Print the formula's dependency tree. Shows the five companion formulae plus their transitive deps. |
| `brew list macontrol` | List every file the formula installed. Includes the binary, the bundled plist template, and the sudoers sample. |
| `brew log macontrol` | git log of the formula in the tap. Shows formula bumps over time. |
| `brew home macontrol` | Open the project homepage in your browser (`https://github.com/amiwrpremium/macontrol`). |

## Tap management

You'll rarely need these directly — `brew install
amiwrpremium/tap/macontrol` taps the repo automatically on the
first install. Reach for these when you're cleaning up or
troubleshooting tap-level issues.

| Command | What it does |
|---|---|
| `brew tap amiwrpremium/tap` | Explicitly add the tap. Equivalent to cloning `amiwrpremium/homebrew-tap` into Homebrew's tap directory. Subsequent installs can use `brew install macontrol` (no tap prefix needed). |
| `brew tap` | List every tap currently active on this Mac. |
| `brew untap amiwrpremium/tap` | Remove the tap. Run after `brew uninstall macontrol` if you're sure you won't reinstall. |
| `brew update` | Refresh every tap's formula index, including `amiwrpremium/tap`. Run before `brew upgrade` if you want the freshest formula. |

## What's NOT a `brew` command

These run via the `macontrol` binary, not via `brew`:

```bash
macontrol setup                              # write Keychain entries (token + whitelist)
macontrol doctor                             # capability + brew-deps + sudoers report
macontrol whitelist {list,add,remove,clear}
macontrol token {set,clear,reauth}
macontrol service {install,uninstall,start,stop,status,logs}
```

The `macontrol service …` family exists for users who installed via
the curl script and don't have `brew services` available. If you
installed via brew, **prefer `brew services …`** — both touch the
same plist, but `brew services` is the supported lifecycle surface
for brew installs. See [CLI](cli.md) for the full subcommand
reference.

## Common workflows

### Fresh install on a new Mac

```bash
brew install amiwrpremium/tap/macontrol
macontrol setup                              # token + whitelist into Keychain
brew services start macontrol
```

### Upgrade to the latest release

```bash
brew update
brew upgrade macontrol
brew services restart macontrol              # load the new binary
```

### Roll back to a previous version

```bash
brew uninstall macontrol
brew install amiwrpremium/tap/macontrol@v0.1.0
brew services restart macontrol
```

### Clean uninstall

```bash
macontrol token clear                        # wipe token from Keychain
macontrol whitelist clear                    # wipe whitelist
brew services stop macontrol                 # bootout the LaunchAgent
brew uninstall macontrol                     # remove the binary
brew untap amiwrpremium/tap                  # optional, if not reinstalling
```

See [Configuration → File locations](../configuration/file-locations.md)
for the full uninstall checklist (logs, plist, sudoers entry).
