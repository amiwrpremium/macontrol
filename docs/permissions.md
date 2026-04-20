# Permissions

macOS gates certain operations behind **TCC** (Transparency, Consent, and
Control) prompts. These can't be bypassed headlessly — the user must grant
them once, interactively, in System Settings → Privacy & Security.

| TCC area | Required for | How to grant |
|---|---|---|
| **Screen Recording** | `/screenshot`, `📸 Media → Screenshot / Record` | Run the action once, then toggle `macontrol` on in System Settings → Privacy & Security → Screen Recording. Restart macontrol after the toggle. |
| **Accessibility** | App listing, fallback brightness via F1/F2 key codes | System Settings → Privacy & Security → Accessibility → add `macontrol` binary. |
| **Camera** | `📸 Media → Webcam photo` (imagesnap) | Run `macontrol doctor` and then the Photo action once. Grant in System Settings → Privacy & Security → Camera. |
| **Automation** | `osascript` calls that target other apps (quit, list running) | The first time the daemon scripts another app, macOS prompts. Click Allow. |

## Sudoers

A handful of macOS commands need `sudo`:

- `pmset` — read-only in most cases, but commonly invoked by the daemon
- `shutdown` — `/power` destructive actions
- `wdutil info` — Wi-Fi diagnostics
- `powermetrics` — thermal samples
- `systemsetup` — timezone set, NTP sync

Rather than granting blanket `sudo`, `macontrol setup` offers to install a
narrow `/etc/sudoers.d/macontrol` with `NOPASSWD` for only those five
binaries. You can also install it manually from the template at
[`sudoers.d/macontrol.sample`](../sudoers.d/macontrol.sample) using
`sudo visudo -f /etc/sudoers.d/macontrol`.
