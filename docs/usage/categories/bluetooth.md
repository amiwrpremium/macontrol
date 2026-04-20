# 🔵 Bluetooth

Toggle the Bluetooth radio, list paired devices, connect/disconnect.
Backed by `blueutil` (auto-installed when macontrol is installed via
Homebrew; manual installs need `brew install blueutil`).

## Dashboard

```text
🔵 Bluetooth — on

[ ⏻ Turn off ] [ 📋 Paired devices ]
[ 🔄 Refresh ]
[        🏠 Home                 ]
```

Header: `on` or `off` — the radio's current state.

## Buttons

### Turn on / Turn off

Toggles via `blueutil --power 1/0`. Header flips.

### Paired devices

Lists every paired device (connected or not) with a one-tap connect or
disconnect button per row:

```text
🔵 Bluetooth Devices

Tap a device to toggle connection.

[ ✂ AirPods Pro       ]     (currently connected — tap to disconnect)
[ 🔗 Magic Keyboard   ]     (currently disconnected — tap to connect)
[ 🔗 Magic Mouse      ]
[ ← Back              ]
[ 🏠 Home             ]
```

The scissors (✂) and link (🔗) icons indicate current state. Tapping
runs `blueutil --connect <mac>` or `--disconnect <mac>` and redirects
back to the device list with updated state.

### Refresh

Re-reads radio state; re-renders the header.

### 🏠 Home

Edits to the inline home grid.

## What's backing this

`blueutil` (brew) wraps the private `CoreBluetooth` framework. The
bot never touches Bluetooth private APIs directly.

| Action | Command |
|---|---|
| Read state | `blueutil -p` (returns `0` or `1`) |
| Toggle | `blueutil --power 0/1` |
| List paired | `blueutil --paired --format json` |
| Connect | `blueutil --connect <addr>` |
| Disconnect | `blueutil --disconnect <addr>` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#bluetooth).

## Edge cases

### `blueutil` not installed

Without the brew formula, every button returns an error:

```text
🔵 Bluetooth — `blueutil` not installed?

⚠ blueutil: exec: "blueutil": executable file not found in $PATH
```

**Fix**: `brew install blueutil`, then tap the category again.

### Trying to connect a non-paired device

`blueutil --connect <addr>` fails with "Device is not paired". macontrol
only shows paired devices in the picker, so this shouldn't happen unless
you tap an expired shortmap entry — see below.

### Expired device short-id

Each paired device's MAC address is stored in an in-memory shortmap
(TTL 15 minutes). If you open the Paired list, wait 16 minutes, then
tap a device, the shortmap entry is gone and you get:

```text
Session expired; refresh the device list.
```

**Fix**: tap Paired devices again. The list is re-built from blueutil,
shortmap entries re-issued.

### Bluetooth off

If the radio is off, `blueutil --paired` returns an empty list. The
Paired view shows "No paired devices." Turn the radio on first.

### Audio device switching

Connecting AirPods via macontrol doesn't automatically route audio to
them — macOS does that based on its own rules (usually "switch to the
new device for a few minutes after connecting"). To explicitly route
audio, use the macOS Sound preference pane.

## Version gates

None — `blueutil` works on macOS 11+. No additional macOS version gates
for this category.
