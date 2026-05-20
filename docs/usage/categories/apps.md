# 🪟 Apps

List the Mac's running user-facing applications and act on them
individually (Quit, Force Quit, Hide) or in bulk via a "Quit all
except…" multi-select. All of it backed by `osascript` talking to
System Events plus `kill -KILL` for the SIGKILL path — no brew
deps.

Requires Accessibility TCC (same grant the System → Top processes
panel uses; granted once, sticky thereafter).

## Dashboard

```text
🪟 Apps  ·  Page 1/1  ·  4 running

[ 🪟 Finder ]
[ 🪟 Mail ]
[ 🪟 · Safari ]
[ 🪟 Visual Studio Code ]
[ 🚮 Quit all except…       ]
[ 🔄 Refresh ] [ ← Back ]
[          🏠 Home          ]
```

The list is alphabetical (case-insensitive), matching Activity
Monitor and the Force Quit menu. The leading "·" before Safari
above marks an app whose windows are currently hidden (Cmd-H
state).

Long lists paginate at 15 entries per page; Prev/Next buttons
appear above the action rows when more than one page is needed.

## Buttons

### Tap an app row

Drills into the per-app action panel:

```text
🪟 Safari · PID 1234

[ 🛑 Quit       ] [ 💀 Force Quit ]
[ 🙈 Hide                          ]
[ 🔄 Refresh ] [ ← Back to apps ]
[             🏠 Home             ]
```

The PID lets you cross-check against Activity Monitor before
pulling the trigger on Force Quit.

#### 🛑 Quit (graceful)

Confirmation gate:

```text
🛑 Quit Safari?

Graceful — the app may show an unsaved-document dialog and
stay open.

[ ✅ Confirm   ] [ ✖ Cancel ]
```

Confirm runs `osascript -e 'tell application "Safari" to quit'`.
The app gets a chance to save state and may refuse the quit if
it has an unsaved-document dialog up — that's by design (matches
Cmd-Q from the keyboard). The bot returns to the list with a
"🛑 Quit sent to Safari." banner above the header so you can
verify what happened.

Cancel returns to the per-app panel (so you can pick Force Quit
instead, not back to the full list).

#### 💀 Force Quit (SIGKILL)

Stronger confirmation gate:

```text
💀 Force Quit Safari (PID 1234)?

Sends SIGKILL — the app can't clean up. Unsaved work is gone.

[ ✅ Confirm   ] [ ✖ Cancel ]
```

Confirm runs `kill -KILL <pid>` against the resolved PID. Hits
the exact instance even when you have multiple copies of the
same app open. List re-renders with a "💀 SIGKILL sent to
Safari (PID 1234)." banner.

#### 🙈 Hide (Cmd-H)

No confirmation — the action is reversible (Cmd-Tab brings the
app's windows back). Runs:

```text
osascript -e 'tell application "System Events" to set visible
of process "<name>" to false'
```

Toasts "Hidden." and re-renders the per-app panel so you can
keep acting on the same app.

### 🚮 Quit all except… (multi-select)

Opens the bulk-quit checklist:

```text
🚮 Quit all except…

Tap apps to keep. Will quit 4 of 4.

[ ✗ Finder ]
[ ✗ Mail ]
[ ✗ Safari ]
[ ✗ Visual Studio Code ]
[ ✅ Quit 4 apps                ]
[ ✖ Cancel                       ]
[          🏠 Home              ]
```

Default is "every app marked QUIT" (the "✗") — matching the
feature name and the realistic ratio (you usually want to keep
2–3 of 30, not the other way around). Tap a row to flip it to
KEEP ("✓"). Tap again to flip back.

The footer button label updates per tap: "Quit 4 apps" → "Quit
3 apps" → … and turns into a disabled "Nothing to quit" label
when you mark every row KEEP.

When you tap "Quit N apps" you get a final summary:

```text
🚮 Quit 3 apps?

Will quit:
  • Finder
  • Safari
  • Visual Studio Code

Will keep:
  • Mail

[ ✅ Yes, quit them ] [ ✖ Cancel ]
```

Confirm runs the per-app graceful Quit serially against every
to-quit name (one `osascript` round-trip per app, in
alphabetical order). The list re-renders with a "🚮 Sent quit to
3 apps." banner. Cancel returns to the checklist with the same
selection so you can adjust without starting over.

No special handling for Telegram or Finder — they appear in the
checklist like everything else. You're trusted to know what
you're doing. (Finder will auto-relaunch if you quit it, which
is by design at the macOS level.)

### 🔄 Refresh

Re-runs the listing osascript and re-renders. Useful after you
launch or quit something from the Mac's UI and want the
dashboard to catch up.

### ← Back / 🏠 Home

← Back returns to the home grid. 🏠 Home does the same; both
buttons exist for a uniform navigation pattern across every
nested screen — see
[Getting started → first message](../../getting-started/first-message.md).

## What's backing this

| Action | Command |
|---|---|
| List | `osascript -e 'tell application "System Events" set out to "" repeat with p in (processes whose background only is false) set out to out & (name of p) & "\|" & (unix id of p) & "\|" & ((not (visible of p)) as text) & linefeed end repeat return out end tell'` |
| Quit | `osascript -e 'tell application "<name>" to quit'` |
| Force Quit | `kill -KILL <pid>` |
| Hide | `osascript -e 'tell application "System Events" to set visible of process "<name>" to false'` |

The listing filter (`processes whose background only is false`)
mirrors what macOS's Force Quit menu and Activity Monitor's
"User interface" view show — agents and daemons aren't
listed.

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md)
for the full table.

## Edge cases

### App refuses to quit (unsaved dialog)

AppleScript's `quit` is non-blocking and returns no error when
the app has an unsaved-document dialog up. The bot reports
`Quit sent to <name>.` even though the app is still running. A
follow-up Refresh confirms the app is still in the list; tap it
again and pick **💀 Force Quit** to bypass the dialog.

### App exits between list-render and tap

If the app you tapped has already exited by the time the per-app
panel renders, the bot shows `🪟 *<name>* — not running anymore.`
Tap **← Back to apps** to re-list. The same applies to the
Force Quit confirm path — the PID lookup re-runs on confirm
to make sure the SIGKILL targets the right process (or no
process at all if the app exited between confirm and execute).

### Multi-select session expires

The "Quit all except…" selection state is parked in a 15-minute
TTL side table (see [Architecture → Design decisions](../../architecture/design-decisions.md)).
A checklist left untouched for 15+ minutes loses its selection
state; the next tap shows "session expired — re-open the
checklist". Just tap **🚮 Quit all except…** from the list
again to start fresh.

### App with a `|` or quote in its name

App names round-trip through the listing's `|`-delimited output;
a name containing `|` would parse-fail and silently drop the
row from the list (the rest of the listing is unaffected). Names
with quotes or backslashes round-trip safely through both the
listing parser and the AppleScript-builder for Quit / Hide
(escapes are applied automatically).

In practice no real Mac app has any of these characters in its
name; this is a defensive note, not a known issue.

## Version gates

None. Every action in this category works on macOS 11+ without
any brew deps. Accessibility TCC is the only requirement.

## Permissions

| Action | Needs |
|---|---|
| Anything in this category | Accessibility TCC |

The Accessibility grant is the same one **🖥 System → Top 10
processes** uses — if you've already granted it, this category
works immediately. If not, the first list-attempt prompts you to
grant it via System Settings → Privacy & Security →
Accessibility → toggle `macontrol` on. Restart the daemon after
toggling.
