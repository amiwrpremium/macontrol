# Categories

Every category on the home keyboard gets its own doc below. Each doc
follows the same shape:

1. **What the dashboard looks like** — a rendering of the message +
   inline keyboard you see after tapping the category.
2. **Every button** — what it does, what it depends on (brew formulae,
   TCC, sudoers), and its edge cases.
3. **Flows** — the multi-step conversations available in this category.
4. **Version gates** — which buttons hide on older macOS releases.

## Quick reference

| Category | Buttons | Flows | Brew deps |
|---|---|---|---|
| [🔊 Sound](sound.md) | −5 / −1 / Mute / +1 / +5 / Set / Max / Refresh | Set exact volume | none |
| [💡 Display](display.md) | −10 / −5 / +5 / +10 / Set / Screensaver / Refresh | Set exact brightness | `brightness` |
| [🔋 Battery](battery.md) | Refresh / Health | — | none |
| [📶 Wi-Fi](wifi.md) | Toggle / Info / Join / DNS x3 / Speedtest / Refresh | Join network (SSID→password) | none (Speedtest needs macOS 12+) |
| [🔵 Bluetooth](bluetooth.md) | Toggle / Paired / Connect / Disconnect | — | `blueutil` |
| [⚡ Power](power.md) | Lock / Sleep / Restart / Shutdown / Logout / Keep-awake / Cancel-awake | Keep-awake minutes | none (destructive ones need confirm) |
| [🖥 System](system.md) | Info / Temp / Mem / CPU / Top / Kill | Kill by pid/name | `smctemp` (optional) |
| [📸 Media](media.md) | Screenshot / Silent shot / Record / Photo | Record duration | `imagesnap` (for Photo) |
| [🔔 Notify](notify.md) | Send / Say | Send (title→body), Say (text) | `terminal-notifier` (preferred, optional) |
| [🛠 Tools](tools.md) | Clipboard get/set / Timezone / Sync / Disks / Shortcut | Clip set, Timezone, Shortcut | none (Shortcut needs macOS 13+) |

## State-ful vs action menus

Dashboards fall into two visual patterns:

**State-ful** (Sound, Display, Battery, Wi-Fi, Bluetooth) — the message
header shows current state and updates every time you tap a button.
Example: Sound's header always ends with `N% · muted/unmuted`.

**Action menus** (Power, System, Media, Notify, Tools) — the message
is a list of actions; tapping each either performs the action
(screenshot, lock) or opens a flow (kill-process, notify-send). The
message itself doesn't display "state".

Battery straddles both. The header shows `percent · state · time
remaining`; the menu has Refresh and Health.

## Navigation

Every leaf dashboard ends with a 🏠 Home row. Tapping it edits the
current message to the inline home grid, from which you can pick any
category again without re-summoning the reply keyboard.

Destructive actions in the Power category show a confirm sub-keyboard
first. See [UX model → Confirm sub-keyboards](../ux-model.md#5-confirm-sub-keyboards-destructive-actions).
