# 🎵 Music

Player-agnostic now-playing metadata + transport control + live progress
for any macOS app that publishes Now Playing info — Music.app, Spotify,
Podcasts, browser-based players, etc. Backed by the third-party
[`nowplaying-cli`](https://github.com/kirtan-shah/nowplaying-cli) brew
formula, which wraps Apple's private `MediaRemote.framework`.

## Dashboard

```text
[ ARTWORK PHOTO ]

🎵 Mr Brightside
_The Killers_ · `Hot Fuss`

Passed: 1:03
▰▰▰▰▰▰▱▱▱▱▱▱
Remaining: 2:39

🔊 60% · unmuted

[ ⏮ Prev ] [ ⏯ Pause ] [ ⏭ Next ]
[ −5 ] [ −1 ] [ 🔇 Mute ] [ +1 ] [ +5 ]
[ ⏩ Seek… ] [ 🔄 Refresh ]
[ ← Back ] [ 🏠 Home ]
```

The dashboard is a Telegram **photo** message — the artwork sits above
the text caption, and the caption holds the metadata, the 3-line
progress block, the volume footer, and the inline keyboard. While
you're on this dashboard the bot edits the caption every 5 s so the
Passed / Remaining / progress bar stay in sync with the player.

When the active track changes (you tap ⏭, or the player advances on
its own), the bot swaps the photo via `editMessageMedia` so the
artwork tracks the song. When `nowplaying-cli` reports no artwork
(some podcast apps, browser audio), a small purple placeholder PNG
is used instead.

## Buttons

### ⏮ Prev / ⏭ Next

Skip to the previous / next track in the player's queue. Per the
macOS / iOS convention `previous` rewinds to the start of the current
track when more than ~3 seconds have elapsed; otherwise it goes to
the actual previous track.

### ⏯ Play / ⏯ Pause

The middle playback button flips between Play and Pause based on
the player's reported playback rate. Tapping invokes the matching
`nowplaying-cli` verb. The bot doesn't track state itself — it
asks the player.

### −5, −1, 🔇 Mute / 🔈 Unmute, +1, +5 (embedded sound)

Identical to the [🔊 Sound](sound.md) dashboard's volume row,
embedded here so you can silence playback without bouncing back to
Sound. The Mute button label flips with the system mute state.

These buttons live on the **mus:** namespace (not snd:) so navigating
away from Music actually fires a non-music callback that stops the
live-refresh goroutine.

### ⏩ Seek… (flow)

Opens a typed-input flow:

```text
Bot: Seek to how many seconds from the start? (0-86400). Reply /cancel to abort.
You: 60
Bot: ⏩ Jumped to 1:00.
```

Accepts any integer 0–86400 (24 h cap). Out-of-range and non-integer
inputs re-prompt without cancelling.

### 🔄 Refresh

Forces an immediate caption re-render rather than waiting up to 5 s
for the next tick.

### 🏠 Home

Edits the message back to the inline home grid. The live-refresh
goroutine stops automatically.

## What's backing this

Every action shells out to `nowplaying-cli`:

- Metadata (per tick):
  `nowplaying-cli get title album artist duration elapsedTime playbackRate contentItemIdentifier`
- Metadata + artwork (first render and on track change):
  `nowplaying-cli get … artworkData`
- Verbs: `nowplaying-cli play|pause|togglePlayPause|next|previous`
- Seek: `nowplaying-cli seek <seconds>`

The output is positional (one value per requested field, one line each)
rather than `--json` because the positional contract is tighter — empty
lines map cleanly to Go zero-values, no JSON number-vs-string union to
handle.

## Refresh behavior

The live caption refresh runs as a per-chat goroutine. It starts when
you tap 🎵 Music from the home grid and stops when:

- You navigate away (any non-music callback fires).
- 10 minutes have elapsed since the session started (hard cap so a
  forgotten chat doesn't spam edits forever).
- The daemon shuts down.

Tick cadence is 5 s — well under Telegram's 1-edit-per-second-per-
message rate limit. Each tick reads metadata + sound state and edits
the caption; only on track-id change does it re-fetch the full
artwork and swap the photo.

## Edge cases

### No track playing

Header reads `🎵 *Music* — _Nothing playing_` (literal Telegram
markdown emitted by the bot). Buttons stay live so you can hit
⏯ Play to resume the player's queue.

### Track without artwork

The dashboard uses the bundled placeholder PNG (a 200×200 flat
purple). All other behavior is unchanged.

### Live streams (zero-duration tracks)

The progress block disappears (no Passed / bar / Remaining lines)
since there's nothing to compute against. Title + artist + volume
footer still render.

### Player exits mid-session

`nowplaying-cli get` keeps returning the last-known metadata for a
short window after the player closes; the bot keeps editing until
that window ends, after which the caption shows "Nothing playing"
on the next tick.

### Track titles with markdown metacharacters

Underscores in artist names ("tame_impala") and similar markdown
metachars are escaped so the rendered caption doesn't accidentally
italicise the rest of the line.

## Version gates

None. The category is gated on the binary-presence flag
`Features.NowPlaying` (set when `nowplaying-cli` is on PATH), not
on a macOS version. The Mac will need at least the macOS version
that the formula supports — currently macOS 11+.

## Install

```bash
brew install nowplaying-cli
```

`macontrol`'s Homebrew formula declares `nowplaying-cli` as a
dependency, so installing macontrol via brew pulls it in
automatically. Otherwise the install reminder shows up in the
dashboard until the binary lands on PATH.

## Permissions

None beyond what the player itself needs. `nowplaying-cli` reads
from the system-wide Now Playing session — no TCC prompts, no
sudoers entry, no Keychain access.
