# Permissions

macontrol controls macOS, so it has to ask macOS for permission. Two
distinct mechanisms:

## What's here

- **[TCC grants](tcc.md)** — Apple's privacy framework. Screen
  Recording, Accessibility, Camera, Automation. User-interactive
  prompts you grant once.
- **[Sudoers](sudoers.md)** — narrow `NOPASSWD` entry for five
  binaries that need root (`pmset`, `shutdown`, `wdutil info`,
  `powermetrics`, `systemsetup`).

## At a glance

| Action | Needs |
|---|---|
| `/lock`, `/sleep` | nothing |
| `/restart`, `/shutdown`, `/logout` | Automation TCC (osascript → System Events) |
| `/screenshot`, `/record` | Screen Recording TCC |
| `📸 Webcam photo` | Camera TCC + `imagesnap` brew |
| `🌡 Temperature` | Sudoers (`powermetrics`) |
| `📶 Wi-Fi → Info` | Sudoers (`wdutil info`) |
| `🧭 Timezone…`, `🔄 Sync time` | Sudoers (`systemsetup`, `sntp`) |
| Listing running apps (osascript) | Accessibility TCC |
| `🎵 Music` (any action) | `nowplaying-cli` brew formula on PATH |
| All other actions | nothing |

## What happens if a permission is missing

| Mechanism | Symptom |
|---|---|
| TCC | macOS prompts the user once. If denied, future attempts silently fail with cryptic errors (e.g. screenshot returns a black image). |
| Sudoers | `sudo -n` fails immediately because no TTY → "a password is required". macontrol surfaces this as an error in the dashboard message. |

Both have specific troubleshooting paths — see
[Troubleshooting → Permission issues](../troubleshooting/permission-issues.md).

## Granting all permissions

The setup wizard prompts you for sudoers automatically. TCC grants
have to be done interactively in System Settings the first time each
permission-needing action is invoked — there's no way to grant them
ahead of time.

The recommended sequence:

1. `macontrol setup` — accept the sudoers offer.
2. After the daemon starts, send `/screenshot` from Telegram. macOS
   prompts for **Screen Recording**. Click *Open System Settings*,
   toggle `macontrol` on, restart the daemon.
3. Send `/start` and tap **🖥 System → Top 10 processes** (or any
   action that lists running apps). macOS prompts for **Accessibility**.
   Same dance.
4. Tap **📸 Media → Webcam photo**. macOS prompts for **Camera**.
   Same dance.
5. Tap **⚡ Power → 🔁 Restart → ✅ Confirm** (then **✖ Cancel**!).
   First time you confirm, macOS prompts for **Automation**. Same
   dance.

After this one-time setup, every TCC grant is sticky and you don't
think about them again.

## Revoking permissions

Open System Settings → Privacy & Security, find the relevant section
(Screen Recording, Accessibility, Camera, Automation), and toggle
`macontrol` off. The change is immediate; the next attempt fails.

Sudoers: `sudo rm /etc/sudoers.d/macontrol` removes the narrow entry.
The five sudo-needing actions then fail with "password required".
