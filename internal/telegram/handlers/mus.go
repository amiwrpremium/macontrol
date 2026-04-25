package handlers

import (
	"bytes"
	"context"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
	"github.com/amiwrpremium/macontrol/internal/telegram/musicrefresh"
)

// handleMusic is the Music dashboard's callback dispatcher.
// Reached via the [callbacks.NSMusic] namespace from any tap on
// the 🎵 Music menu and from the embedded sound nudges.
//
// Routing rules (data.Action — first match wins):
//  1. "open" / "refresh" → first-render or manual refresh path.
//     "open" deletes the prior text dashboard and sendPhoto's a
//     fresh photo + caption + keyboard, then starts the
//     [musicrefresh.Manager] for this chat. "refresh" forces an
//     immediate tick on the existing session.
//  2. "play" / "pause" / "toggle" → matching nowplaying-cli verb;
//     [musicrefresh.Manager.Refresh] re-renders immediately.
//  3. "next" / "prev" → matching nowplaying-cli verb;
//     editMessageMedia fires on the next refresh when TrackID
//     changes.
//  4. "seek" → installs [flows.NewSeek] for typed seconds.
//  5. "vol-up" / "vol-down" → adjust volume by ±delta via
//     [sound.Service.Adjust]. delta defaults to 5; data.Args[0]
//     overrides.
//  6. "vol-mute" / "vol-unmute" → matching [sound.Service]
//     toggle. Embedded so the user doesn't have to leave Music
//     to silence playback.
//
// All unknown actions fall through to a "Unknown music action."
// toast.
//
// Capability gate: every action checks Features.NowPlaying first
// and surfaces an install-reminder toast when nowplaying-cli is
// missing. The "open" path renders the install reminder as the
// dashboard's only screen instead.

// musicDispatch maps Music callback actions to per-action handlers.
// "open" and "refresh" share a renderer; "vol-*" actions are
// embedded sound controls.
var musicDispatch = map[string]func(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error{
	"open":       handleMusicOpen,
	"refresh":    handleMusicRefresh,
	"play":       handleMusicPlay,
	"pause":      handleMusicPause,
	"toggle":     handleMusicToggle,
	"next":       handleMusicNext,
	"prev":       handleMusicPrev,
	"seek":       handleMusicSeek,
	"vol-up":     handleMusicVolNudge,
	"vol-down":   handleMusicVolNudge,
	"vol-mute":   handleMusicVolMute,
	"vol-unmute": handleMusicVolUnmute,
}

func handleMusic(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	h, ok := musicDispatch[data.Action]
	if !ok {
		Reply{Deps: d}.Toast(ctx, q, "Unknown music action.")
		return nil
	}
	return h(ctx, d, q, data)
}

// handleMusicOpen is the entry path from the home grid. Deletes
// the prior text message (the home dashboard) and replaces it
// with a sendPhoto carrying the artwork + caption + keyboard.
// Then starts the live-refresh manager for this chat.
//
// When the nowplaying-cli binary is absent, edits the current
// message in place to an install-reminder text rather than
// converting to a photo — there's no artwork to show and no
// reason to swap the message type.
func handleMusicOpen(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	if !d.Capability.Features.NowPlaying {
		r.Ack(ctx, q)
		text := keyboards.MusicCaption(music.NowPlaying{}, soundStateOrZero(ctx, d), false)
		kb := keyboards.MusicKeyboard(false, false, false)
		return r.Edit(ctx, q, text, kb)
	}
	r.Ack(ctx, q)
	chatID := q.Message.Message.Chat.ID
	priorMsgID := q.Message.Message.ID

	np, err := d.Services.Music.GetWithArtwork(ctx)
	if err != nil {
		return errEdit(ctx, r, q, "🎵 *Music* — unavailable", err)
	}
	vol, _ := d.Services.Sound.Get(ctx)

	// Replace the text dashboard with a photo message. Prior
	// message gets deleted so the chat doesn't accumulate dead
	// home-grid messages.
	_, _ = d.Bot.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: priorMsgID,
	})

	art := np.Artwork
	if len(art) == 0 {
		art = musicrefresh.Placeholder
	}

	caption := keyboards.MusicCaption(np, vol, true)
	kb := keyboards.MusicKeyboard(np.IsPlaying(), vol.Muted, true)
	msg, err := d.Bot.SendPhoto(ctx, &tgbot.SendPhotoParams{
		ChatID: chatID,
		Photo: &models.InputFileUpload{
			Filename: "artwork.png",
			Data:     bytes.NewReader(art),
		},
		Caption:     caption,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
	if err != nil {
		return err
	}
	d.MusicRefresh.Start(ctx, chatID, msg.ID, np)
	return nil
}

// handleMusicRefresh forces an immediate tick on the existing
// refresher session. Used by the explicit "🔄 Refresh" button
// and as the post-action re-render path for play / pause / etc.
//
// When no session is active (e.g. user tapped Refresh outside
// of an Open call, or the session expired), falls back to the
// open path.
func handleMusicRefresh(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	if !d.Capability.Features.NowPlaying {
		return handleMusicOpen(ctx, d, q, data)
	}
	chatID := q.Message.Message.Chat.ID
	if !d.MusicRefresh.IsActive(chatID) {
		return handleMusicOpen(ctx, d, q, data)
	}
	r.Ack(ctx, q)
	d.MusicRefresh.Refresh(ctx, chatID)
	return nil
}

// handleMusicPlay / handleMusicPause / handleMusicToggle /
// handleMusicNext / handleMusicPrev all share the same shape:
//   - capability check; toast on miss.
//   - call the matching music.Service verb.
//   - immediate re-render via [musicrefresh.Manager.Refresh].
//
// The verb dispatchers are tiny but kept separate (rather than
// folded into a verb-keyed sub-dispatch) so the routing rules
// table reads 1:1 with the dispatch map.

func handleMusicPlay(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicVerb(ctx, d, q, "▶ play failed", d.Services.Music.Play)
}

func handleMusicPause(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicVerb(ctx, d, q, "⏸ pause failed", d.Services.Music.Pause)
}

func handleMusicToggle(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicVerb(ctx, d, q, "⏯ toggle failed", d.Services.Music.TogglePlayPause)
}

func handleMusicNext(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicVerb(ctx, d, q, "⏭ next failed", d.Services.Music.Next)
}

func handleMusicPrev(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicVerb(ctx, d, q, "⏮ previous failed", d.Services.Music.Previous)
}

// musicVerb is the shared shell for play/pause/toggle/next/prev.
// Acks the query, capability-gates, runs the verb, then forces
// a refresher tick so the user sees the post-action state.
func musicVerb(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, errLabel string, verb func(context.Context) error) error {
	r := Reply{Deps: d}
	if !d.Capability.Features.NowPlaying {
		r.Toast(ctx, q, "Install nowplaying-cli first.")
		return nil
	}
	r.Ack(ctx, q)
	if err := verb(ctx); err != nil {
		r.Toast(ctx, q, errLabel)
		return err
	}
	d.MusicRefresh.Refresh(ctx, q.Message.Message.Chat.ID)
	return nil
}

// handleMusicSeek installs the typed-seconds Seek flow.
func handleMusicSeek(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	r := Reply{Deps: d}
	if !d.Capability.Features.NowPlaying {
		r.Toast(ctx, q, "Install nowplaying-cli first.")
		return nil
	}
	r.Ack(ctx, q)
	chatID := q.Message.Message.Chat.ID
	f := flows.NewSeek(d.Services.Music)
	d.FlowReg.Install(chatID, f)
	return sendFlowPrompt(ctx, r, chatID, f.Start(ctx))
}

// handleMusicVolNudge wraps sound.Service.Adjust for the
// embedded ±5 / ±1 buttons. delta defaults to 5; data.Args[0]
// overrides. Down actions sign-flip the delta.
func handleMusicVolNudge(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, data callbacks.Data) error {
	r := Reply{Deps: d}
	delta := 5
	if len(data.Args) > 0 {
		if v, err := strconv.Atoi(data.Args[0]); err == nil {
			delta = v
		}
	}
	if data.Action == "vol-down" {
		delta = -delta
	}
	r.Ack(ctx, q)
	if _, err := d.Services.Sound.Adjust(ctx, delta); err != nil {
		r.Toast(ctx, q, "🔊 adjust failed")
		return err
	}
	d.MusicRefresh.Refresh(ctx, q.Message.Message.Chat.ID)
	return nil
}

// handleMusicVolMute / handleMusicVolUnmute mirror the [Sound]
// dashboard's mute toggles, kept on the music namespace so
// the refresher stays alive.

func handleMusicVolMute(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicMuteToggle(ctx, d, q, true)
}

func handleMusicVolUnmute(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, _ callbacks.Data) error {
	return musicMuteToggle(ctx, d, q, false)
}

// musicMuteToggle calls Sound.Mute or Sound.Unmute then
// forces an immediate refresher tick.
func musicMuteToggle(ctx context.Context, d *bot.Deps, q *models.CallbackQuery, mute bool) error {
	r := Reply{Deps: d}
	r.Ack(ctx, q)
	var err error
	if mute {
		_, err = d.Services.Sound.Mute(ctx)
	} else {
		_, err = d.Services.Sound.Unmute(ctx)
	}
	if err != nil {
		r.Toast(ctx, q, "🔊 mute toggle failed")
		return err
	}
	d.MusicRefresh.Refresh(ctx, q.Message.Message.Chat.ID)
	return nil
}

// soundStateOrZero is a Get-with-fallback helper for the
// install-reminder render path where Sound is informational
// only and a missing read shouldn't tank the whole render.
func soundStateOrZero(ctx context.Context, d *bot.Deps) (s sound.State) {
	if d == nil || d.Services.Sound == nil {
		return s
	}
	got, err := d.Services.Sound.Get(ctx)
	if err != nil {
		return s
	}
	return got
}
