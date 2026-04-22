# 🖥 System

OS and hardware info, thermal pressure, memory and CPU summaries,
top-N processes, kill. Mostly read-only; `Kill` is the one destructive
action.

°C readings in the Temperature panel use `smctemp` (auto-installed when
macontrol is installed via Homebrew; manual installs need
`brew install smctemp`). Everything else in this category uses built-in
macOS commands.

## Dashboard

```text
🖥 System

[ ℹ Info    ] [ 🌡 Temperature ]
[ 🧠 Memory ] [ ⚙ CPU         ]
[ 📋 Top 10 processes ] [ 🔪 Kill process… ]
[          🏠 Home               ]
```

Each action opens its own panel (edits the message to that panel) with
a Refresh button and a Back-to-menu button.

## Buttons

### Info

Panel:

```text
🖥 System info

• macOS 15.3.1 (24D70)
• Host: tower.local
• Model: MacBookPro18,3
• Chip: Apple M3 Pro
• Cores: 11 (6 performance and 5 efficiency)
• RAM: 32 GiB
• Uptime: 6 days, 3h 14m
• Logged-in users: 4
• Load avg (1/5/15m): 0.92 / 0.87 / 0.85 (~8% / 8% / 8% of 11 cores)
```

The load averages are macOS's scheduler run-queue averages over the
last 1, 5, and 15 minutes. As a rule of thumb, sustained values above
**1.0 per core** mean the CPU is saturated; below that, there's
headroom. The percentage is `load ÷ cores × 100` and is purely a
convenience — it can briefly exceed 100% under burst.

If `uptime`'s output can't be parsed (unexpected format), macontrol
falls back to showing the raw line on a single bullet.

Sources:

| Line | Command |
|---|---|
| OS version | `sw_vers` |
| Host | `hostname` |
| Model | `sysctl -n hw.model` |
| Chip | `sysctl -n machdep.cpu.brand_string` |
| Cores | `system_profiler SPHardwareDataType` → `Total Number of Cores` |
| RAM | `sysctl -n hw.memsize` |
| Uptime / users / load avg | `uptime` (parsed into separate bullets) |

Best-effort: if any subprocess fails, the panel still renders with the
fields that did succeed. Only fails entirely if **all** of them fail.

### Temperature

Panel:

```text
🌡 Thermal

• Pressure: Nominal
• CPU: 52.7°C
• GPU: 47.1°C
```

Two signals:

- **Pressure** — one of `Nominal`, `Moderate`, `Heavy`, `Trapping`,
  `Sleeping`. Reported by `sudo powermetrics --samplers thermal`.
  Requires the sudoers entry. If sudo fails, pressure shows `unknown`.
- **°C** — from `smctemp -c` (CPU) and `smctemp -g` (GPU). Requires
  `brew install smctemp`. Without it, the lines show "°C readings
  unavailable (install `brew install smctemp`)".

Apple Silicon doesn't expose detailed per-sensor thermal readings via
public APIs, so macontrol's temperature view is less rich than
Intel-era tools. See
[Architecture → Design decisions](../../architecture/design-decisions.md#why-no-fan-control-or-per-sensor-thermal)
for why.

### Memory

Panel:

```text
🧠 Memory

PhysMem: 18G used (2G wired), 6G unused.

• The system has 70% of its memory free.
```

Sources: `top -l 1 -s 0` for the PhysMem line, `memory_pressure` for
the pressure summary, `vm_stat` captured in the raw output.

### CPU

Panel:

```text
⚙ CPU

• CPU usage: 5.10% user, 3.20% sys, 91.70% idle
• 10:22 up 6 days, 3:14, 4 users, load averages: 0.92 0.87 0.85
```

Sources: `uptime` for load average, `top -l 1 -s 0` CPU usage line.

### Top 10 processes

Monospace table of the 10 highest-CPU processes:

```text
📋 Top 10 by CPU
```
```text
PID     %CPU  %MEM  CMD
100     12.5   3.2  /Applications/App.app
205      8.7   5.1  /usr/bin/python3
341      6.0   1.0  /System/Library/PrivateFrameworks/…/WindowServer
…
```

Runs `ps -Ao pid,pcpu,pmem,comm -r | head -n 11`.

Snapshot only — not live. Tap Refresh to re-fetch.

### Kill process… (flow)

```text
Bot: Send a PID (integer) or a process name to kill. Reply /cancel to abort.
You: 341
Bot: ✅ SIGTERM sent to pid 341.
```

Or by name:

```text
You: Safari
Bot: ✅ killall Safari done.
```

PID path uses `kill <pid>` (SIGTERM only — no SIGKILL).
Name path uses `killall <name>`.

Errors (no such process, permission denied) are reported in-line and
the flow ends.

There is no undo. The bot will kill anything it can kill with your
user's permissions.

### 🏠 Home

Edits to the inline home grid.

## Edge cases

### Temperature pressure is "unknown"

Means `sudo powermetrics` failed — usually missing sudoers entry. See
[Permissions → Sudoers](../../permissions/sudoers.md) to install the
narrow entry via the setup wizard.

### smctemp reports different values between runs

Known issue on M2+ chips — the SMC sensor values can oscillate
slightly between samples. Treat the number as a rough guide, not a
precise measurement.

### Killing privileged processes

`kill`/`killall` can't terminate processes owned by other users or by
root. System daemons (launchd, WindowServer, etc.) are protected. If
the kill fails, the error is shown.

### Top 10 vs Activity Monitor

Activity Monitor's CPU% is normalized per-core; `ps -r`'s is
percentage of a single core. So a fully-busy multi-threaded process
can show >100% in `ps` on macontrol but <100% in Activity Monitor.
Same data, different unit.

## Version gates

None for Info, Memory, CPU, Top, Kill.

Temperature pressure readings work on macOS 11+, but the `powermetrics`
output format evolves — macontrol parses defensively but very old or
very new formats may misparse. `smctemp` is brew and compatible across
supported versions.
