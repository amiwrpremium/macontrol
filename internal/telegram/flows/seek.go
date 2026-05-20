package flows

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
)

// NewSeek returns the typed-seconds [Flow] that jumps the
// current track to the user's exact position via
// [music.Service.Seek].
//
// Behavior:
//   - Asks for an integer second count, validates it's in
//     [0, 86400] (24 h cap matches the longest realistic single
//     audio file — most podcasts and audiobook chapters fit).
//   - Re-prompts on parse failure or out-of-range value without
//     terminating the flow (the user gets to retry).
//   - One-shot once a valid value is supplied: terminates after
//     the Seek call (Done=true on both success and failure).
//
// Wire it into the chat with [Registry.Install] from
// [handlers.handleMusic] when the user taps "⏩ Seek…" on the
// 🎵 Music dashboard.
func NewSeek(svc *music.Service) Flow {
	return &seekFlow{svc: svc}
}

// seekFlow is the [NewSeek]-returned [Flow]. Holds only the
// [music.Service] reference; one-shot.
type seekFlow struct {
	svc *music.Service
}

// Name returns the dispatcher log identifier "mus:seek".
func (seekFlow) Name() string { return "mus:seek" }

// Start emits the typed-input prompt with the [0, 86400]
// range hint.
func (seekFlow) Start(_ context.Context) Response {
	return Response{Text: "Seek to how many seconds from the start? (`0`-`86400`). Reply `/cancel` to abort."}
}

// Handle parses the integer second count and dispatches to
// [music.Service.Seek].
//
// Routing rules (first match wins):
//  1. text fails to parse as int OR is < 0 OR > 86400 →
//     "Please reply with a whole number of seconds, 0..86400."
//     (NOT terminal — re-prompted).
//  2. Seek returns non-nil err → "⚠ could not seek: `<err>`" +
//     Done.
//  3. Otherwise → "⏩ Jumped to `<m:ss>`." + Done. The reported
//     position is the value the user typed (NOT a re-fetched
//     player state) because some players reject seeks silently
//     on non-seekable content; the bot reports the requested
//     position rather than confusingly showing the unchanged
//     state.
func (f *seekFlow) Handle(ctx context.Context, text string) Response {
	secs, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || secs < 0 || secs > 86400 {
		return Response{Text: "Please reply with a whole number of seconds, 0..86400."}
	}
	if err := f.svc.Seek(ctx, secs); err != nil {
		return Response{Text: fmt.Sprintf("⚠ could not seek: `%v`", err), Done: true}
	}
	return Response{
		Text: fmt.Sprintf("⏩ Jumped to `%s`.", formatSeekPosition(secs)),
		Done: true,
	}
}

// formatSeekPosition renders secs as m:ss (or h:mm:ss for
// values >= 3600). Used by [seekFlow.Handle] in the success
// banner.
func formatSeekPosition(secs int) string {
	d := time.Duration(secs) * time.Second
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
