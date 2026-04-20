# 📸 Media

Screenshots, screen recording, and webcam photos. Screenshots and
recordings need Screen Recording TCC permission; the webcam needs
Camera TCC.

## Dashboard

```text
📸 Media

Screenshots and screen recordings need Screen Recording TCC
permission. Webcam photos need Camera.

[ 📷 Screenshot ] [ 📷 Silent shot ]
[ 📹 Record… ] [ 📸 Webcam photo ]
[         🏠 Home                  ]
```

## Buttons

### Screenshot (with shutter sound)

Captures **all displays** into a single PNG, returns it as a Telegram
photo attached to the chat.

Runs `screencapture /tmp/macontrol-screenshot-*.png`. The tempfile is
removed after the upload completes.

**First-time TCC prompt**: macOS asks for **Screen Recording** permission
the first time. Click *Open System Settings*, toggle `macontrol` on,
then tap again. macontrol has to restart after the toggle (the daemon
picks up new TCC state on next launch).

See [Permissions → TCC](../../permissions/tcc.md) for the full walk
through.

### Silent shot

Same as Screenshot but with `-x` so no shutter sound plays. Useful when
your Mac is in a shared space and you don't want to announce the capture.

### Record… (flow)

```text
Bot: Record for how many seconds? (1-120). Reply /cancel to abort.
You: 10
Bot: (nothing for 10 seconds)
Bot: ✅ Recorded 10s.
```

Runs `screencapture -v -V 10 /tmp/macontrol-record-*.mov`. The `-V`
flag (macOS 14+) gives it a duration cap. After the recording
completes, the MOV is uploaded as a Telegram video.

Constraints:

- 1–120 seconds. Under 1 or over 120 re-prompts without cancelling.
- Telegram bot API has a **50 MB** upload limit. A 1080p/30fps
  screen-record is roughly 5–10 MB per second, so practically 5–8
  seconds max before Telegram rejects the upload. The bot replies
  with the error if upload fails.

### Webcam photo

Captures a single frame from the built-in FaceTime camera using
`imagesnap -q -w 1 -`. Returns a JPEG.

**Requires**:

- `brew install imagesnap`
- **Camera** TCC permission on first use

Without `imagesnap` installed, the button returns:

```text
⚠ webcam failed: imagesnap: executable file not found in $PATH

Install brew install imagesnap and grant Camera permission.
```

The `-w 1` flag warms the sensor for one second before capture, which
gives a much better exposure than no warmup. Tap-to-result latency is
~2 seconds.

### 🏠 Home

Edits to the inline home grid.

## What's backing this

| Action | Command |
|---|---|
| Screenshot | `screencapture <path>` |
| Silent shot | `screencapture -x <path>` |
| Record | `screencapture -v -V <secs> <path>` |
| Photo | `imagesnap -q -w 1 <path>` |

See [Reference → macOS CLI mapping](../../reference/macos-cli-mapping.md#media).

## Edge cases

### Screen Recording denied by accident

If you click Deny on the TCC prompt, macOS doesn't ask again. The
screenshot just silently fails and returns a blank or corrupt PNG.

**Fix**: System Settings → Privacy & Security → Screen Recording →
check `macontrol` in the list (add it if missing). Restart the daemon
with `macontrol service stop && macontrol service start`.

### Multi-display setups

The default Screenshot captures **all** displays in a single image —
one big PNG with all of them side by side. There's no per-display
button in the UI; if you want only one display, the capability exists
in the domain layer (`screencapture -D 1`) but isn't wired to a button
yet. Open an issue if you want this.

### Record stops early

If the daemon is killed mid-recording (SIGTERM, crash), the MOV may
be truncated or missing the `moov` atom and be unplayable. Telegram
usually still accepts it as a video file even if it won't play.

### Webcam is busy

If another app is using the camera (Zoom, FaceTime, Photo Booth), the
webcam shot returns "camera is already in use by another app". Close
the other app first.

### Camera TCC prompt doesn't appear

Some macOS configurations (MDM-managed Macs, certain corporate
profiles) suppress the Camera prompt. You'll see "operation not
permitted" in the error text. Ask your Mac admin to allow `imagesnap`
in the MDM config.

## Version gates

- Screenshot / Silent shot — macOS 11+
- Record (`-V` duration cap) — macOS 14+ for the exact duration
  behavior; on older releases the duration cap may be ignored and the
  process runs until SIGINT. macontrol enforces the cap via context
  timeout regardless.
- Webcam photo — macOS 11+, needs `imagesnap` brew formula.
