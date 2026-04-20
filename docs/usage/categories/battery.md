# 🔋 Battery

Read-only dashboard of the battery's current state and long-term health.
No brew deps, no sudo.

## Dashboard

```text
⚡ Battery — 78% · charging · 1:12 remaining

[ 🔄 Refresh ] [ 📊 Health ]
[        🏠 Home            ]
```

The emoji in the header shifts based on state:

- ⚡ when charging
- 🔋 when discharging and ≥ 40%
- 🪫 when discharging and < 40%

## Buttons

### Refresh

Re-reads `pmset -g batt`, re-renders the header. Use this if you
plugged/unplugged and want the dashboard to catch up.

### Health

Queries `system_profiler SPPowerDataType` and renders:

```text
🔋 Battery health

• Condition: Normal
• Cycle count: 312
• Maximum capacity: 91%
• Adapter: 70W
```

What each field means:

| Field | Notes |
|---|---|
| Condition | `Normal`, `Service Recommended`, or `Service`. Apple considers anything other than Normal a replace-the-battery signal. |
| Cycle count | Integer. Apple considers laptops designed for 1,000 cycles; M-series Macs are rated higher but anything under 500 is well within spec. |
| Maximum capacity | Current full-charge capacity as a percent of original design capacity. Drops below 80% warrants replacement. |
| Adapter | Wattage of the currently-connected charger, or empty if on battery. |

### 🏠 Home

Edits to the inline home grid.

## What's backing this

- Header: `pmset -g batt` — parsed for percent, state, and time remaining.
- Health: `system_profiler SPPowerDataType` — parsed for cycle count,
  condition, max capacity, and adapter wattage.

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#battery).

## Edge cases

### Desktop Mac (no battery)

On a Mac mini, Mac Studio, Mac Pro, or iMac, `pmset -g batt` emits
either `No batteries available` or `Battery is not present`.

macontrol detects this and renders:

```text
🔋 Battery — not present (desktop Mac)
```

Refresh still works. Health returns an empty report (no fields).

### "Finishing charge" state

When the battery is at 99–100% and still drawing a trickle, pmset
reports `finishing charge`. macontrol treats this as `AC` (full).

### Adapter wattage shows as `0W`

If the charger is unplugged or macOS hasn't read the adapter yet,
`Wattage (W):` line may be missing. In that case, the Adapter field
renders as empty or `W` alone — harmless.

### Cycle count doesn't match System Settings

`system_profiler` and the Settings app sometimes lag each other by a
few cycles. macontrol shows what `system_profiler` reports.

## Version gates

None — both actions work on macOS 11+ without brew deps.

## Why no notifications when battery is low

macontrol is request-response, not push. It doesn't watch the battery
and send alerts when it hits 20%, etc. For that, use a macOS Shortcut
personal automation (Settings app → Shortcuts → Automations) — it has
battery-level triggers.
