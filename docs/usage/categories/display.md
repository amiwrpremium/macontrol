# 💡 Display

Control the built-in display's brightness and launch the screensaver.
External monitors are not supported — see
[Design decisions](../../architecture/design-decisions.md) for why.

Backed by `brightness` (auto-installed when macontrol is installed via
Homebrew; manual installs need `brew install brightness`).

## Dashboard

```text
💡 Display — 70%

[ −10 ] [ −5 ] [ +5 ] [ +10 ]
[       Set exact value…       ]
[ 🌙 Screensaver ] [ 🔄 Refresh ]
[          🏠 Home              ]
```

If the `brightness` brew formula isn't installed, the header reads
`unknown (install brew install brightness for level readings)`. The
buttons still work in that case only if the osascript F1/F2 fallback
path is available — see below.

## Buttons

### ±10, ±5

Relative adjust. Takes the current level, adds/subtracts the delta,
clamps to [0, 1], writes via `brightness <float>`. No wraparound.

Each tap is one `brightness` CLI invocation. The clamp prevents going
below 0 or above 1.

### Set exact value… (flow)

```text
Bot: Enter brightness 0-100. Reply /cancel to abort.
You: 80
Bot: ✅ Brightness set — 80%
```

Accepts integers 0–100. Internally divides by 100 before calling
`brightness` (which wants a 0.0–1.0 float).

### Screensaver

Runs `open -a ScreenSaverEngine`. Starts the screensaver immediately.
Works on every macOS version without any brew deps or TCC prompts.
Move the mouse or press any key to dismiss.

Note: starting the screensaver doesn't lock the screen on its own —
your security settings control whether it requires a password. If you
want to lock, use **[⚡ Power → 🔒 Lock](power.md)** instead.

### Refresh

Re-reads the brightness via `brightness -l` and re-renders the header.

### 🏠 Home

Edits to the inline home grid.

## What's backing this

`brightness` is a brew formula (<https://formulae.brew.sh/formula/brightness>).
It works on Apple Silicon's built-in display.

- Read all displays: `brightness -l` — parses the `display 0: brightness X.Y`
  line for display 0.
- Set: `brightness 0.750` (3-decimal float).

If you want to add support for external monitors via DDC/CI, `ddcctl`
and `betterdisplaycli` are the options — PRs welcome, but it'd need a
UI decision for which display to target when multiple are attached.

## Edge cases

### No `brightness` CLI installed

Without `brew install brightness`, the read path fails and the header
shows `unknown`. The buttons still try to call the CLI and fail — the
error edits the dashboard to show a hint.

**Fix**: `brew install brightness`, then Refresh.

### External display only (no built-in)

If you're on a Mac mini or Mac Studio with only external displays, the
built-in `brightness` CLI won't find a display 0 and returns "unknown".
The screensaver button still works.

### Very dim (0%)

Brightness of 0 does **not** turn the display off — it goes to the
minimum backlight the panel supports, which is usually still visible
in a dark room. To actually black out the screen, use
[⚡ Power → 💤 Sleep](power.md).

## Version gates

None — brightness and screensaver both work on macOS 11+.
