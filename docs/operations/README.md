# Operations

Day-to-day running of the daemon: starting, stopping, watching logs,
checking health, and upgrading.

## What's here

- **[Running](running.md)** — three ways to run macontrol (foreground,
  `brew services`, `launchctl`); restart cycles; signal handling.
- **[Logs](logs.md)** — paths, rotation, levels, and how to read them.
- **[Doctor](doctor.md)** — `macontrol doctor` output explained
  line-by-line.
- **[Upgrades](upgrades.md)** — `brew upgrade` flow, manual upgrade,
  release notes, version pinning.

## At a glance

| Task | Command |
|---|---|
| Start / restart at login | `brew services start macontrol` (or `macontrol service install`) |
| Stop now | `brew services stop macontrol` (or `macontrol service stop`) |
| Tail logs | `tail -f ~/Library/Logs/macontrol/macontrol.log` |
| Status | `brew services info macontrol` (or `macontrol service status`) |
| Health check | `macontrol doctor` |
| Upgrade | `brew upgrade macontrol && brew services restart macontrol` |
| Uninstall | `macontrol service uninstall && brew uninstall macontrol` |

## What "running" actually looks like

```bash
launchctl list | grep macontrol
# 7891  0  com.amiwrpremium.macontrol
```

The first column is the PID; the second is the most recent exit code
(0 means it's running and healthy, or last exit was clean). The third
is the launchd label.

If the PID column shows `-`, macontrol is loaded into launchd but not
currently running — usually because it crashed and is in
`ThrottleInterval` backoff (10 s after a crash before relaunch).

## Process supervision model

The daemon doesn't run forever as a single process; it's restarted by
launchd as needed:

- **At user login** — RunAtLoad fires, daemon starts.
- **On crash** — launchd notices the exit, waits 10 s
  (ThrottleInterval), starts again.
- **On `launchctl bootout`** — clean stop, launchd stops trying.
- **On Mac wake from sleep** — daemon stays running through sleep
  (sleep is a userland-pause, not a process death). Long-poll
  reconnects to Telegram on wake.

This means:

- macontrol auto-recovers from any panic that the recover middleware
  doesn't catch.
- Killing it manually with `kill <pid>` will be reverted by launchd
  10 s later.
- To stop it permanently, use `brew services stop` or `macontrol
  service stop` — both run `launchctl bootout`, which tells launchd
  not to restart.

## Where to look when things break

Order of investigation:

1. **Is the daemon running?** `launchctl list | grep macontrol`. If
   PID is `-`, it's crashing — check logs.
2. **What did the logs say?** `tail -50 ~/Library/Logs/macontrol/macontrol.log`.
   Look for the most recent `ERROR` or `panic`.
3. **Are dependencies OK?** `macontrol doctor`. Highlights missing
   brew deps, broken sudoers, version mismatches.
4. **Is Telegram itself up?** `curl -s "https://api.telegram.org/bot<token>/getMe"`.
5. **Is the host network OK?** `ping 1.1.1.1` and
   `dig api.telegram.org`.

Each of these has a dedicated section in
[Troubleshooting → Common issues](../troubleshooting/common-issues.md).
