# File locations

Where macontrol puts things on macOS, with the rationale for each path.

All paths are macOS-idiomatic — under `~/Library/` rather than home-dir
dotfiles, because that's where macOS apps put per-user state.

## User-specific paths (per-user, no root)

### Config

```text
~/Library/Application Support/macontrol/config.env
```

Permissions: `0600` (rw user only). Created with mode `0700` directory
permissions.

Contents: the env-var format documented in [env.md](env.md).

Why here: macOS's recommended location for per-user app config.
Spotlight indexes it (you can find it via Cmd-Space "macontrol config")
but Time Machine backs it up by default — so your bot config survives
disk-wipes, and migrating to a new Mac picks it up via Migration
Assistant.

### Logs

```text
~/Library/Logs/macontrol/macontrol.log              # current
~/Library/Logs/macontrol/macontrol.log.1.gz         # rotated
~/Library/Logs/macontrol/macontrol.log.2.gz         # rotated
…up to 5 backups, 30-day max age
```

Permissions: `0750` directory, `0644` files.

Rotation: handled by `lumberjack`. Max 10 MB per file, 5 backups, 30
days. Old rotations are gzipped.

Why here: Console.app reads `~/Library/Logs/` automatically — open
Console, navigate to your username, and you'll see macontrol's logs
alongside Apple's app logs.

### LaunchAgent plist

```text
~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist
```

Permissions: `0600`.

Contents: a plist that launchd uses to start the daemon at login and
restart it on crash. See
[Operations → Running](../operations/running.md) for what's in it.

Why here: `~/Library/LaunchAgents/` is the standard location for
per-user agents (vs `/Library/LaunchAgents/` for system-wide ones).
Per-user means it runs as your account, has access to your home dir,
and can produce notifications on your screen.

### Cache (rarely populated)

```text
~/Library/Caches/macontrol/
```

Permissions: `0750`.

Currently empty in normal use. Reserved for future on-disk caches.

## System paths (root-owned)

### Sudoers entry (optional)

```text
/etc/sudoers.d/macontrol
```

Permissions: `0440`, owned by `root:wheel`.

Contents: narrow `NOPASSWD` entries for five binaries — `pmset`,
`shutdown`, `wdutil info`, `powermetrics`, `systemsetup`.

Why here: `/etc/sudoers.d/` is the canonical location for additive
sudoers fragments. They're read in addition to `/etc/sudoers`. macontrol
ships its narrow set as a separate file so you can `rm` it cleanly to
revert.

See [Permissions → Sudoers](../permissions/sudoers.md) for the file
contents and what it grants.

## Binary location

Depends on how you installed:

| Install method | Binary path |
|---|---|
| Homebrew | `/opt/homebrew/bin/macontrol` |
| Manual (curl script, root) | `/usr/local/bin/macontrol` |
| Manual (curl script, non-root) | `~/.local/bin/macontrol` |
| Built from source (`make build`) | `dist/macontrol` (in repo) |

The LaunchAgent plist references the binary by absolute path, so if
you move it after running `macontrol service install`, run
`macontrol service install` again to rewrite the plist.

## Discover everything macontrol writes

To see all macontrol-touched paths:

```bash
ls -la ~/Library/Application\ Support/macontrol/ \
       ~/Library/Logs/macontrol/ \
       ~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist \
       ~/Library/Caches/macontrol/ \
       /etc/sudoers.d/macontrol \
       2>/dev/null
```

## Clean uninstall

Remove the binary and these paths to leave no trace:

```bash
# Stop and unload the LaunchAgent
macontrol service uninstall

# Binary
brew uninstall macontrol           # if installed via brew
# or
sudo rm /usr/local/bin/macontrol   # if installed manually

# Per-user state
rm -rf ~/Library/Application\ Support/macontrol
rm -rf ~/Library/Logs/macontrol
rm -rf ~/Library/Caches/macontrol
rm -f  ~/Library/LaunchAgents/com.amiwrpremium.macontrol.plist

# Sudoers (if installed)
sudo rm /etc/sudoers.d/macontrol

# Optional brew deps you may want to keep
brew uninstall brightness blueutil terminal-notifier smctemp imagesnap
```

After this, `find ~ /etc -name '*macontrol*' 2>/dev/null` should show
nothing.

## TCC permissions are stored separately

The Privacy & Security toggles you grant macontrol (Screen Recording,
Accessibility, Camera) live in the system's TCC database at
`/Library/Application Support/com.apple.TCC/TCC.db` and are not removed
by uninstalling. To revoke, remove the entries via System Settings →
Privacy & Security after uninstalling. See
[Permissions → TCC](../permissions/tcc.md).
