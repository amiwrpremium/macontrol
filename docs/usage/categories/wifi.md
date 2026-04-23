# 📶 Wi-Fi

Toggle Wi-Fi, read diagnostics, join networks, switch DNS, and run a
speed test. No brew deps; some features require sudo or macOS 12+.

## Dashboard

```text
📶 Wi-Fi — on · SSID home · iface en0
WPA2 Personal · -45 dBm · 144 Mbps · ch 2g3/20

[ ⏻ Turn off ] [ ℹ Info ]
[        🔗 Join network…        ]
[ 🌐 DNS → Cloudflare ] [ 🌐 DNS → Google ] [ 🌐 DNS → DHCP ]
[ ⚡ Speed test ]       (only on macOS 12+)
[ 🔄 Refresh  ]
[ ← Back     ] [ 🏠 Home ]
```

Header (line 1):

- `on` / `off` — whether the Wi-Fi radio is enabled
- `SSID <name>` — name of the associated network, or `(not associated)`
  if disconnected, or `—` if Wi-Fi is off
- `iface <name>` — the Wi-Fi interface name (usually `en0`, sometimes
  `en1` on Macs with multiple network ports)

Rich link details (line 2 — only when associated AND wdutil/system_profiler
returned data):

- **Security** — e.g. `WPA2 Personal`, `WPA/WPA2 Personal`, `Open`
- **Signal** — RSSI in dBm (closer to 0 = stronger; -45 is very strong,
  -75 is borderline)
- **Tx Rate** — current PHY link rate in Mbps
- **Channel** — e.g. `2g3/20` from wdutil (band/number/width) or
  `3 (2GHz, 20MHz)` from system_profiler

The second line is omitted entirely if no rich data is available (Wi-Fi
off, not associated, or both `wdutil` and `system_profiler` couldn't
be queried).

### How SSID is read

Since macOS 14.4 Apple restricted `networksetup -getairportnetwork` —
it returns `"You are not associated with an AirPort network"` even when
connected, unless the calling process has Location permission. macontrol
sidesteps this by reading SSID (and the rich fields above) from
`sudo wdutil info` (already in macontrol's narrow sudoers entry).
If the sudoers entry isn't installed, falls back to
`system_profiler SPAirPortDataType` (slower, ~2-3s, no sudo). If both
fail, SSID stays empty and the dashboard shows `(not associated)`.

## Buttons

### Turn on / Turn off

Toggles `networksetup -setairportpower en0 on/off`. The button label
flips based on current state.

After toggling, the header updates to show the new state.

### Info

Runs `sudo wdutil info` and returns the full output in a code block —
a dedicated diagnostics drill-down panel:

```text
📶 Wi-Fi diagnostics
```
…raw wdutil text…
```

[ 🔄 Refresh ] [ ← Back ]
[        🏠 Home         ]
```

Includes BSSID, channel, signal strength, noise, security type, etc.
Refresh re-runs wdutil; Back returns to the main Wi-Fi dashboard.

**Requires** the narrow sudoers entry — see
[Permissions → Sudoers](../../permissions/sudoers.md). Without it,
sudo prompts for a password, which the daemon can't answer, and the
button returns an error.

### Join network… (flow)

Two-step flow:

```text
Bot: Send the SSID to join. Reply /cancel to abort.
You: MyNetwork
Bot: Now send the password for MyNetwork. Send - for an open network.
You: mypassword123
Bot: ✅ Joined — SSID MyNetwork · iface en0
```

The `-` password means open (no auth). Empty passwords are rejected
and re-prompt.

If the join fails (wrong password, network not found, out of range),
the bot replies with the error message from `networksetup`.

### DNS presets

Three one-tap presets:

- **Cloudflare** — `1.1.1.1` + `1.0.0.1`
- **Google** — `8.8.8.8` + `8.8.4.4`
- **DHCP** (reset) — clears manual DNS, back to whatever DHCP hands you

Applies via `networksetup -setdnsservers Wi-Fi <servers or "Empty">`.

After tapping, the dashboard header updates with a small `_DNS updated
→ <preset>_` suffix. The suffix disappears on the next refresh.

### Speed test (macOS 12+)

Hidden on macOS 11. On macOS 12+:

```text
⚡ Speedtest

• Down: 523.4 Mbps
• Up:   87.2 Mbps
```

Runs `networkQuality -v` (which ships with macOS 12+). Takes about 15
seconds. The bot shows "Running — takes ~15s…" as a toast while the
measurement is in flight.

Note: `networkQuality` measures end-to-end TCP/UDP throughput against
Apple's test servers — not Wi-Fi link speed. Numbers reflect your
internet connection, not your local Wi-Fi RF.

### Refresh

Re-reads interface, power, and SSID; re-renders the header.

### 🏠 Home

Edits to the inline home grid.

## What's backing this

| Action | Command |
|---|---|
| Interface discovery | `networksetup -listallhardwareports` (parse "Wi-Fi" or "AirPort" port) |
| Power state | `networksetup -getairportpower en0` |
| Current SSID + rich link details | `sudo wdutil info` (preferred); `system_profiler SPAirPortDataType` (fallback). `networksetup -getairportnetwork` is **not used** — see "How SSID is read" above |
| Toggle | `networksetup -setairportpower en0 on/off` |
| Join | `networksetup -setairportnetwork en0 <ssid> [<password>]` |
| Info diagnostics dump | `sudo wdutil info` (raw text) |
| DNS set | `networksetup -setdnsservers Wi-Fi <servers or Empty>` |
| Speed test | `networkQuality -v` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#wi-fi).

## Edge cases

### No Wi-Fi interface

Some Macs (Mac mini with Ethernet-only configuration, Mac Pro) can
have no Wi-Fi port. In that case, the bot returns "no Wi-Fi hardware
port found" and every action in this category fails.

### Legacy "AirPort" naming

Older macOS versions call it "AirPort" instead of "Wi-Fi". macontrol
accepts both when discovering the hardware port.

### Scanning for available networks

**Not supported.** Apple removed the `airport -s` command in macOS
14.4 and hasn't provided a public replacement. `wdutil` doesn't scan.

Workaround: use the macOS menu bar Wi-Fi icon to see available networks,
then tap **🔗 Join network…** with the SSID you want.

See [Architecture → Design decisions](../../architecture/design-decisions.md)
for more on why scanning is intentionally out of scope.

### Captive portals

macontrol can join a network but cannot automate captive-portal logins
(hotel Wi-Fi, coffee shop Wi-Fi). After joining, open the Wi-Fi menu
on the Mac manually to accept the portal.

### Tailscale / VPN on top of Wi-Fi

The header's SSID reflects the underlying Wi-Fi link, not any VPN
tunnel. Speedtest also measures the VPN path, which is usually slower
than bare Wi-Fi.

## Version gates

| Feature | Min macOS |
|---|---|
| Toggle, Info, Join, DNS, Refresh | 11.0 |
| Speed test | 12.0 (Monterey) |
| `wdutil info` output format | 11.0 (format has changed across releases; macontrol returns the raw text) |
