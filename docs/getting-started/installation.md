# Installation

macontrol ships as a single Go binary for `darwin/arm64`. Pick one of
three installation paths below. Homebrew is the most convenient for
everyday use; the manual script is for air-gapped or script-driven
setups; building from source is for contributors.

## Before you start

Confirm your Mac is compatible:

```bash
uname -sm
# Expected: Darwin arm64
```

If this returns anything other than `Darwin arm64`, stop — macontrol
targets Apple Silicon only and the daemon will refuse to run on Intel
Macs or other platforms.

Check your macOS version:

```bash
sw_vers -productVersion
```

Must be `11.0` or higher. Some features need newer releases — see
[Reference → Version gates](../reference/version-gates.md).

## Option 1 — Homebrew (recommended)

One command installs the binary, the LaunchAgent template, and the
sudoers sample, and pulls them all into `/opt/homebrew`:

```bash
brew install amiwrpremium/tap/macontrol
```

If you've never tapped the repo before, Homebrew auto-taps it for you.
To tap explicitly:

```bash
brew tap amiwrpremium/tap
brew install macontrol
```

### What got installed

- Binary: `/opt/homebrew/bin/macontrol`
- LaunchAgent template: `/opt/homebrew/opt/macontrol/launchd/com.amiwrpremium.macontrol.plist`
- Sudoers sample: `/opt/homebrew/opt/macontrol/sudoers.d/macontrol.sample`
- Five companion brew formulae, pulled in as hard dependencies of the
  `macontrol` formula:

| Formula | Unlocks |
|---|---|
| `brightness` | Exact display brightness readings and control |
| `blueutil` | Bluetooth toggle, paired-device list, connect/disconnect |
| `terminal-notifier` | Rich desktop notifications (sounds, icons) |
| `smctemp` | CPU/GPU °C readings on Apple Silicon |
| `imagesnap` | Webcam photo capture |

No action required — `brew install amiwrpremium/tap/macontrol` installs
all of them in one shot. They're tracked by Homebrew and updated
alongside macontrol.

If you go the manual install path (Option 2) instead, you'll want to
install these yourself:

```bash
brew install brightness blueutil terminal-notifier smctemp imagesnap
```

Without them, the buttons that depend on them return "unavailable"
error messages instead of crashing. `macontrol doctor`
(see [Operations → Doctor](../operations/doctor.md)) prints which deps
are missing.

### Next

Run the first-time setup wizard:

```bash
macontrol setup
```

See [Quickstart](quickstart.md) for what the wizard asks and does.

## Option 2 — Manual install via script

Useful if you're not using Homebrew or if you're scripting an install
into another tool's setup.

```bash
curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | sh
```

The script:

1. Refuses to run unless the host is `darwin/arm64`.
2. Resolves the latest release tag via the GitHub API.
3. Downloads `macontrol_<version>_darwin_arm64.tar.gz`.
4. Verifies its SHA-256 against the published `checksums.txt`.
5. Extracts and installs the binary to `/usr/local/bin` (if writable)
   or `~/.local/bin` (with a reminder to add it to `PATH`).

If you want to audit the script first:

```bash
curl -fsSL https://raw.githubusercontent.com/amiwrpremium/macontrol/master/scripts/install.sh | less
```

### Next steps

```bash
macontrol setup
macontrol service install      # writes LaunchAgent plist; launchctl-loads it
```

## Option 3 — Build from source

For contributors, or if you want to run a modified version.

```bash
git clone https://github.com/amiwrpremium/macontrol.git
cd macontrol
make build                     # cross-compiles for darwin/arm64
```

Output: `dist/macontrol`. Move it onto your `PATH`:

```bash
install -m 0755 dist/macontrol /usr/local/bin/macontrol
```

You can also build for the local host (useful if you're iterating on a
non-darwin dev box):

```bash
make build-local
```

See [Development → Contributing](../development/contributing.md) for
the full developer workflow.

## Verify the install

```bash
macontrol --version
```

Expected output:

```text
macontrol v0.1.0 (abc1234, 2026-04-20)
```

If `command not found`, the install directory isn't on your `PATH`.
Homebrew installs don't have this issue; manual installs to
`~/.local/bin` often do.

```bash
# Add to PATH for zsh (the default shell on macOS 10.15+):
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Uninstalling

**Homebrew**:

```bash
brew services stop macontrol
brew uninstall macontrol
```

**Manual**:

```bash
macontrol service uninstall    # removes LaunchAgent + stops the daemon
rm -f /usr/local/bin/macontrol
rm -rf ~/Library/Application\ Support/macontrol
rm -rf ~/Library/Logs/macontrol
```

To also remove the narrow sudoers entry:

```bash
sudo rm /etc/sudoers.d/macontrol
```

## Next step

→ [Telegram credentials](credentials-telegram.md) — create the bot and
get your user ID before running `macontrol setup`.
