# Usage

macontrol is **menu-first**, not command-first. The slash commands
exist as muscle-memory shortcuts; the bot's main UI is the keyboard
that appears below your input field after you send `/menu`.

This group covers every interactive surface:

## What's here

- **[UX model](ux-model.md)** — how the keyboards behave: one-shot
  reply keyboards, inline keyboards that edit messages in place,
  multi-step flows for free-text input, confirm sub-keyboards for
  destructive actions.
- **[Slash commands](commands.md)** — `/start`, `/menu`, `/status`,
  `/help`, `/cancel`, `/lock`, `/screenshot`. Quick reference with
  examples.
- **[Categories](categories/README.md)** — every button in every
  category dashboard, with what it does, what it depends on, and the
  edge cases.

## At a glance

The bot has three distinct interaction shapes:

| Shape | Example | When you'll see it |
|---|---|---|
| **Reply keyboard** | The 12-button home keyboard with categories | After `/start` or `/menu` |
| **Inline keyboard** | `−5 / −1 / MUTE / +1 / +5` under a 🔊 Sound message | When you tap a category, or navigate within one |
| **Flow** | Bot asks "Enter target volume (0-100)" and waits | When you tap an action that needs free-text input |

Reply keyboards collapse after one tap (one-shot mode). Inline
keyboards stay attached to their message and edit the message text
when you press a button. Flows take over the chat for up to 5 minutes
or until you reply with `/cancel`.

## What you can't do (by design)

- **No arbitrary shell** — there is no `/sh <cmd>`. Every action is a
  named, vetted command. If you want broader access, author a macOS
  Shortcut and call it via the **🛠 Tools → ⚡ Run Shortcut…** flow.
- **No multi-tenant config** — single owner, single whitelist. Each
  whitelisted user gets the same access; there are no roles.
- **No fan-out** — the bot doesn't broadcast. `/notify` from one user
  doesn't notify the others. The boot ping is the only message the
  daemon sends unprompted.
- **No control of other Macs** — one daemon controls one Mac.

For the *why* behind these choices, see
[Architecture → Design decisions](../architecture/design-decisions.md).
