# Troubleshooting

Quick lookup for things that can break, organized by symptom.

## What's here

- **[Common issues](common-issues.md)** — startup failures, "bot
  doesn't reply", "button does nothing".
- **[Permission issues](permission-issues.md)** — TCC and sudoers
  errors with exact log lines.
- **[Telegram errors](telegram-errors.md)** — Bot API HTTP codes
  (401, 403, 429, 5xx).

## First steps when something doesn't work

1. **Run `macontrol doctor`** — most issues are missing brew deps,
   missing sudoers, or wrong macOS version.
2. **Check the log**:
   ```bash
   tail -50 ~/Library/Logs/macontrol/macontrol.log
   ```
3. **Confirm the daemon is running**:
   ```bash
   launchctl list | grep macontrol
   ```
4. **Confirm the daemon is the version you think**:
   ```bash
   macontrol --version
   ```
5. **Match the error message** against the docs in this group. Most
   common errors are listed verbatim in [Common issues](common-issues.md).

If none of that helps, file a bug report with the doctor output and
recent logs (with your user IDs redacted) — see
[Support](../../SUPPORT.md).
