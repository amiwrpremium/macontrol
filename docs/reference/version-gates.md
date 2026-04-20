# Version gates

Some macontrol features require newer macOS releases. The daemon
detects the current version at startup and hides or marks-unavailable
features that aren't supported.

## How detection works

`internal/capability/detect.go`:

1. `runner.Exec("sw_vers", "-productVersion")` returns something like
   `"15.3.1\n"`.
2. `ParseVersion` splits on `.`, parses the major/minor/patch as ints.
3. `DeriveFeatures` sets boolean flags based on minor-version checks.
4. The result is stored in `bot.Deps.Capability.Features` and consulted
   by handlers and keyboards before rendering version-gated buttons.

## The features

| Feature | Min macOS | What it gates |
|---|---|---|
| `WdutilInfo` | 11.0 | `📶 Wi-Fi → Info` (always-true on 11+, exists for symmetry) |
| `NetworkQuality` | 12.0 (Monterey) | `📶 Wi-Fi → Speed test` |
| `Shortcuts` | 13.0 (Ventura) | `🛠 Tools → Run Shortcut…` |

Only three gates today. Most macontrol features work on the entire
ASi-supported range (macOS 11+).

## Hidden vs. unavailable

When a feature is gated:

- **Keyboard layer** hides the button entirely. Cleaner UX — the user
  doesn't see something they can't tap.
- **Handler layer** also rejects the action with a toast ("Speedtest
  needs macOS 12+") in case the user constructs the callback some
  other way (replayed message, bot test).

Defense in depth.

## What this looks like in code

In a keyboard builder (`internal/telegram/keyboards/wif.go`):

```go
if features.NetworkQuality {
    rows = append(rows, []models.InlineKeyboardButton{
        {Text: "⚡ Speed test", CallbackData: callbacks.Encode(callbacks.NSWifi, "speedtest")},
    })
}
```

In a handler (`internal/telegram/handlers/wif.go`):

```go
case "speedtest":
    if !feat.NetworkQuality {
        r.Toast(ctx, q, "Speedtest needs macOS 12+")
        return nil
    }
```

## Boot summary

The daemon emits a one-line capability report at startup:

```text
INFO  msg=capabilities  summary="macOS 15.3 · 3/3 version-gated features available"
```

The numerator is how many of the gated features your macOS supports;
the denominator is the total number of gates (currently 3). On macOS
11.7 you'd see `1/3`; on 12.0+ `2/3`; on 13.0+ `3/3`.

`macontrol doctor` shows the same data with each feature spelled out:

```text
macOS:            15.3
networkQuality:   true
shortcuts CLI:    true
wdutil info:      true
```

## Adding a new gate

When you add a feature that needs a specific macOS version:

1. Edit `internal/capability/detect.go`:
   - Add a bool field to `Features`:
     ```go
     type Features struct {
         NetworkQuality bool
         Shortcuts      bool
         WdutilInfo     bool
         MyNewThing     bool   // add this
     }
     ```
   - Add the gate in `deriveFeatures`:
     ```go
     return Features{
         NetworkQuality: v.AtLeast(12, 0),
         Shortcuts:      v.AtLeast(13, 0),
         WdutilInfo:     v.AtLeast(11, 0),
         MyNewThing:     v.AtLeast(14, 5),
     }
     ```
2. Update the `count` helper so the boot summary numerator matches:
   ```go
   func (f Features) count() (available, total int) {
       flags := []bool{f.NetworkQuality, f.Shortcuts, f.WdutilInfo, f.MyNewThing}
       …
   }
   ```
3. In the relevant keyboard, gate the button on `features.MyNewThing`.
4. In the relevant handler, reject the action if `!feat.MyNewThing`.
5. Add a row to this file.
6. Update `macontrol doctor` output (`cmd/macontrol/doctor.go`) to
   print the new flag.

## Why version-gate instead of try-and-fail

Two reasons:

- **Better UX** — a hidden button is clearer than a button that returns
  "command not found". The user knows immediately that they're missing
  the prerequisite.
- **Fewer log warnings** — every "command not found" exits the
  subprocess with an error and gets logged. Gating prevents the noise
  for things we can statically know don't exist.

The cost is one bool field per feature. Worth it.

## What about brew deps?

Brew dependencies are runtime-detected (every invocation does an
`exec.LookPath` or just runs the command and handles the not-found
error). They're not version-gated because:

- Brew installs are independent of macOS versions.
- A user can install or remove a brew formula at any time.
- Re-running `macontrol doctor` shows the current state.

Version gates are for things that are baked into the OS and won't
change without a macOS upgrade.
