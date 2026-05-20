# Sudoers

A handful of macOS commands need root. Rather than asking you for a
password every time (impossible from a daemon with no TTY), macontrol
ships a **narrow** sudoers entry that grants `NOPASSWD` access to
exactly five binaries — nothing more.

## What it grants

The installed `/etc/sudoers.d/macontrol` looks like:

```sudoers
# /etc/sudoers.d/macontrol
# Narrow passwordless-sudo entry for the macontrol daemon.
# Only the five binaries the bot actually needs; no blanket ALL.

amiwrpremium  ALL=(root) NOPASSWD: /usr/bin/pmset
amiwrpremium  ALL=(root) NOPASSWD: /usr/sbin/shutdown
amiwrpremium  ALL=(root) NOPASSWD: /usr/bin/wdutil info
amiwrpremium  ALL=(root) NOPASSWD: /usr/bin/powermetrics
amiwrpremium  ALL=(root) NOPASSWD: /usr/sbin/systemsetup
```

(`amiwrpremium` is replaced with whatever Unix username `macontrol
setup` was run under.)

That's it. Five binaries. No wildcards. No `ALL=(ALL) NOPASSWD: ALL`
catastrophe.

## What it doesn't grant

Specifically:

- **Not arbitrary commands** — `sudo ls /root` still prompts for
  password. You can't smuggle other commands through.
- **Not other binaries with the same name** — `/usr/bin/pmset` is
  pinned by absolute path. A trojan `pmset` you put on `$PATH`
  wouldn't get sudo access.
- **Not subcommands of the listed binaries** unless the line allows
  them. The `wdutil info` line allows only the `info` subcommand —
  `sudo wdutil reset` would prompt for a password.
- **Not other users** — only the user the wizard ran as is on the
  list.

## Why each binary

| Binary | Used for |
|---|---|
| `/usr/bin/pmset` | Reads detailed power state and could set sleep behavior. macontrol uses it for `pmset -g batt` (battery info — actually doesn't need sudo, but the entry is there for symmetry) and `pmset sleepnow` (also doesn't need sudo, see note below). |
| `/usr/sbin/shutdown` | Restart/shutdown that doesn't go through AppleScript (currently macontrol uses AppleScript instead, but the entry is reserved for a future force-shutdown feature). |
| `/usr/bin/wdutil info` | Wi-Fi diagnostics (BSSID, channel, signal, noise). The `info` subcommand needs root since macOS 14.4. |
| `/usr/bin/powermetrics` | Thermal pressure samples. Needs root because it reads kernel-level performance counters. |
| `/usr/sbin/systemsetup` | Reading and setting timezone, NTP sync. The interactive `Date & Time` UI runs with root privileges; the CLI needs sudo. |

A few of these (`pmset` for read, `pmset sleepnow`) don't strictly
need sudo today, but the wizard installs them anyway because:

- macOS occasionally tightens privilege requirements. If a future macOS
  version adds sudo to `pmset sleepnow`, the entry already covers it.
- Symmetry is easier to audit than "some need sudo, some don't".

## Installing the entry

### Via the wizard (recommended)

```bash
macontrol setup
# …
# ▸ Install narrow sudoers entry (shutdown/pmset/wdutil/powermetrics/systemsetup)? [y/N] y
#   (will run `sudo visudo -cf /etc/sudoers.d/macontrol` to validate)
# ▸ /etc/sudoers.d/macontrol written  ✓
```

The wizard:

1. Generates the file content (with your username substituted).
2. Writes it to a tempfile.
3. Validates with `sudo visudo -cf <tempfile>`. If validation fails,
   the install is aborted — you can't accidentally lock yourself out
   of sudo.
4. Installs to `/etc/sudoers.d/macontrol` with mode `0440` and owner
   `root:wheel`.

The `sudo` calls happen in the wizard itself, so you'll be prompted
for your password.

### Manually

```bash
# 1. Copy the template from the brew install
cp /opt/homebrew/opt/macontrol/sudoers.d/macontrol.sample /tmp/macontrol-sudoers

# 2. Edit it: replace REPLACE_ME with your Unix username
sed -i '' "s/REPLACE_ME/$USER/g" /tmp/macontrol-sudoers

# 3. Validate
sudo visudo -cf /tmp/macontrol-sudoers
# Expected: /tmp/macontrol-sudoers: parsed OK

# 4. Install
sudo install -m 0440 -o root -g wheel /tmp/macontrol-sudoers /etc/sudoers.d/macontrol
```

The exact same operations the wizard does, just by hand.

### Validation matters

Always run `visudo -cf` before installing a sudoers file. A syntactically
broken sudoers file disables sudo entirely until root fixes it — and
recovering from "I broke sudo" requires booting into Recovery Mode and
mounting your filesystem manually. The `0440` permissions and
`root:wheel` ownership are also required by `sudo`; deviating breaks
the entry silently (`sudo` ignores files with wrong perms).

## Removing the entry

```bash
sudo rm /etc/sudoers.d/macontrol
```

After this, the five sudo-needing macontrol actions fail with "password
required" errors. The bot still runs; just those features become
unavailable.

## What you give up by NOT installing it

- **🌡 System → Temperature** falls back to "unknown" pressure (no
  `powermetrics`).
- **📶 Wi-Fi → Info** returns "permission denied" (no `wdutil info`).
- **🛠 Tools → Timezone…** can't set timezone.
- **🛠 Tools → Sync time** can't run sntp.
- **⚡ Power → Restart / Shutdown / Logout** still works (uses
  AppleScript path, not sudo).

The bot is still useful without sudoers — most features don't need it.
But if you want full thermal + wifi diagnostics + timezone control,
install the entry.

## Verifying the entry is active

After install, test with:

```bash
sudo -n pmset -g batt
```

Should print battery state without prompting. If it prompts for a
password, the entry isn't active. Common causes:

- File permissions are wrong (`stat /etc/sudoers.d/macontrol` should
  show `-r--r----- 1 root wheel`).
- File isn't owned by `root:wheel`.
- Username in the entry doesn't match the user running the test.
- There's a syntax error and `sudo` is silently ignoring the file —
  re-run `sudo visudo -cf /etc/sudoers.d/macontrol`.

`macontrol doctor` runs this same check and reports the result. See
[Operations → Doctor](../operations/doctor.md).

## The "blanket ALL" anti-pattern

You may have seen examples online like:

```sudoers
amiwrpremium ALL=(ALL) NOPASSWD: ALL
```

This grants the user passwordless sudo to **everything**. macontrol
explicitly does not do this, because:

- Any compromise of the daemon → instant root.
- Any compromise of the bot token → instant root.
- An attacker on your whitelist could `sudo` arbitrary commands.

The narrow entry limits the blast radius: even with a compromised bot
or token, an attacker can only invoke the five named binaries with
their pre-defined arguments. They can't `sudo cat /etc/passwd`,
`sudo dscl`, or `sudo rm -rf /`.

## What if I want to extend it

If you fork macontrol and add a feature that needs another sudo-only
binary:

1. Add the binary path + subcommand to `cmd/macontrol/assets.go` →
   `sudoersBody()`.
2. Add a doc note here describing what it grants and why.
3. Run `macontrol setup --reconfigure` (or `sudo visudo -f
   /etc/sudoers.d/macontrol`) on machines that already have the entry
   — additions don't auto-merge.

Resist the urge to broaden to wildcards. Pin the absolute path and
the subcommand if possible.
