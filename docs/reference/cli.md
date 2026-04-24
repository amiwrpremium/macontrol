# CLI

Every subcommand the `macontrol` binary supports.

## Synopsis

```text
macontrol [subcommand] [args...]
```

If no subcommand is given (`macontrol` alone), `run` is implied.

## Subcommands

### `run`

Run the daemon.

```text
macontrol run [--log-level=LEVEL] [--log-file=PATH]
```

Default if no subcommand. Reads the bot token + whitelist from the
Keychain, detects capabilities, starts the long-poll loop.
Foreground; `Ctrl-C` (SIGINT) or SIGTERM stops it cleanly.

Used by the LaunchAgent plist as the `ProgramArguments`.

| Flag | Default | Notes |
|---|---|---|
| `--log-level` | `info` | One of `debug`, `info`, `warn`, `error`. Unknown values silently fall back to `info`. |
| `--log-file` | `~/Library/Logs/macontrol/macontrol.log` | Lumberjack-rotated. Pass an empty string (`--log-file=`) to log to stderr instead. |

**Exit codes**:

- `0` — clean exit (signal received, context cancelled).
- `1` — Keychain entry missing/locked, capability error, or bot init failure.

### `setup`

Interactive first-run wizard.

```text
macontrol setup [--reconfigure]
```

Walks through:

1. Bot token (hidden input + token-validity check via Telegram getMe).
2. Telegram user ID(s).
3. Optional LaunchAgent install + start.
4. Optional narrow sudoers install (with `visudo -cf` validation).
5. TCC permissions reminder.

`--reconfigure` allows re-running over an existing config (otherwise
the wizard refuses to overwrite).

**Exit codes**:

- `0` — wizard completed successfully or user declined to overwrite.
- `1` — token validation failed, user input invalid, file write error.

### `service`

LaunchAgent management.

```text
macontrol service install
macontrol service uninstall
macontrol service start
macontrol service stop
macontrol service status
macontrol service logs
```

| Action | Effect |
|---|---|
| `install` | Generate plist with current binary path, write to `~/Library/LaunchAgents/`, `launchctl bootstrap`. Implies `start`. |
| `uninstall` | `launchctl bootout` then remove the plist. |
| `start` | `launchctl bootstrap gui/$UID …` (no plist write). |
| `stop` | `launchctl bootout gui/$UID/com.amiwrpremium.macontrol`. |
| `status` | `launchctl print gui/$UID/com.amiwrpremium.macontrol`. |
| `logs` | `tail -n 200 -f` on `~/Library/Logs/macontrol/macontrol.log`. |

**Exit codes**:

- `0` — operation succeeded.
- `1` — invalid subcommand.
- non-zero from `launchctl` for command failures.

### `doctor`

Health report.

```text
macontrol doctor
```

Prints capability detection, brew dependency presence, and sudoers
reachability. See [Operations → Doctor](../operations/doctor.md) for
sample output.

Always exits `0` (it's a report, not an enforcement tool).

### `whitelist`

Manage the user-ID whitelist stored in the Keychain.

```text
macontrol whitelist list
macontrol whitelist add <userid>
macontrol whitelist remove <userid>
macontrol whitelist clear
```

| Action | Effect |
|---|---|
| `list` | Print every whitelisted Telegram user ID, one per line. Empty list prints `(empty)`. |
| `add <id>` | Append the integer to the whitelist. Idempotent (no-op if already present). |
| `remove <id>` | Drop the integer. Refuses to remove the last entry — use `clear` for that. Aliased as `rm`. |
| `clear` | Empty the whitelist after explicit `[y/N]` confirmation. |

After any add / remove / clear, restart the daemon for it to pick up
the change (`brew services restart macontrol`).

**Exit codes**:

- `0` — operation succeeded
- `1` — invalid argument or Keychain error
- `2` — missing required subcommand

### `token`

Manage the bot token stored in the Keychain.

```text
macontrol token set
macontrol token clear
macontrol token reauth
```

| Action | Effect |
|---|---|
| `set` | Hidden-input prompt for a new token, validates via Telegram `getMe`, replaces the Keychain entry. Aborts on validation failure. |
| `clear` | Remove the token after confirmation. The daemon will refuse to start until a new one is set. |
| `reauth` | Read the existing token and re-issue the Keychain entry with a fresh `-T <macontrol-binary>` ACL. Use after the binary moved (brew bottle relocation, switched install methods). |

After `set`, restart the daemon. After `reauth`, no restart needed —
the next read is silent.

**Exit codes**: `0` on success, `1` on Keychain or validation error,
`2` on missing subcommand.

### `version` / `--version` / `-v`

Print version metadata.

```text
macontrol version
macontrol --version
macontrol -v
```

Output:

```text
macontrol v0.1.0 (abc1234, 2026-04-20)
```

The three values come from link-time variables:

- `Version` — semver tag (or `dev` for local builds)
- `Commit` — short git SHA (or `none` for local builds)
- `Date` — UTC build date (or `unknown`)

Exit code: `0`.

### `help` / `--help` / `-h`

Print the subcommand list.

```text
macontrol help
macontrol --help
macontrol -h
```

Exit code: `0`.

## Flags

Two flags on the `run` subcommand (`--log-level`, `--log-file`); the
`setup` subcommand has `--reconfigure`. Everything else is pure
subcommand dispatch — no global flags.

There are intentionally no env vars that change daemon behavior.
Secrets live in the macOS Keychain (written by `macontrol setup`),
runtime knobs are CLI flags. See
[Configuration → Runtime](../configuration/runtime.md).

## Environment variable interaction

The CLI subcommands consult only a handful of process env vars, all
related to filesystem layout:

| Variable | Used by | Effect |
|---|---|---|
| `HOME` | `setup`, `service`, `run` | Resolves where to put plist / logs. |
| `USER` | `setup` | Used in sudoers entry generation if `user.Current()` fails. |

macontrol does **not** read `TELEGRAM_BOT_TOKEN`,
`ALLOWED_USER_IDS`, `LOG_LEVEL`, `MACONTROL_CONFIG`, or
`MACONTROL_LOG` — they are all gone. Use Keychain entries (managed
via `macontrol token` / `macontrol whitelist`) and the `--log-level`
/ `--log-file` flags on `run`.

`setup` and `service` write absolute paths into the plist and
sudoers entry, so they need to be invoked **as the user the daemon
will run as** — not via `sudo macontrol setup`. The wizard prompts
for sudo internally only for the file write to `/etc/sudoers.d/`.

## Argv parsing semantics

Pure stdlib — no `cobra`, `kingpin`, or `flag` package. Just `os.Args`
inspection. This keeps startup fast and the binary small.

Implications:

- Flags must come **after** the subcommand: `macontrol setup --reconfigure`,
  not `macontrol --reconfigure setup`.
- Flag-style args before any subcommand (`macontrol --foo`) are
  treated as if you'd run `macontrol run` with that flag — which the
  daemon ignores.
- Unknown subcommands print an error and exit 2.
- Order of `service install` matters — `install` must be the second
  argument.

## Common one-liners

```bash
# Quickest install + setup path
brew install amiwrpremium/tap/macontrol && macontrol setup

# Restart the daemon and tail the new boot logs
brew services restart macontrol && macontrol service logs

# Diagnose with one command
macontrol doctor

# Print version + check it's running
macontrol --version && launchctl list | grep macontrol

# Force-restart without changing the plist
launchctl kickstart -k gui/$UID/com.amiwrpremium.macontrol
```
