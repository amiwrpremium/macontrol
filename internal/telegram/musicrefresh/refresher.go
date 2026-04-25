// Package musicrefresh owns the per-chat live-refresh goroutines
// that keep the 🎵 Music dashboard's photo caption in sync with
// the player's actual position every 5 seconds.
//
// The refresher exists because the bot has no "user is currently
// looking at this message" signal from Telegram — the only way
// to deliver a live progress bar is to edit the message
// periodically until we have a reason to stop. Reasons to stop:
//
//   - The user navigates away (any non-music callback fires —
//     the dispatcher in the handlers package calls
//     [Manager.Stop] for the chat).
//   - The 10-minute hard cap elapses (so a forgotten chat
//     doesn't spam edits forever).
//   - The daemon's parent context is cancelled (shutdown).
//
// Public surface:
//
//   - [Manager] — the per-process singleton stored on
//     bot.Deps.MusicRefresh.
//   - [NewManager] — constructor; bot pointer is wired later via
//     [Manager.SetBot] because [bot.Deps.Bot] is populated only
//     after [tgbot.Bot.Start] runs.
//   - [Manager.Start] / [Manager.Stop] / [Manager.Touch] — the
//     handler-facing API.
//   - [Placeholder] — the embedded fallback PNG used when
//     nowplaying-cli reports no artwork.
package musicrefresh

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// DefaultTickInterval is the per-chat refresh cadence in
// production. 5 s comfortably stays under Telegram's 1-edit-
// per-second-per-message rate limit and matches the user-spec.
const DefaultTickInterval = 5 * time.Second

// DefaultMaxDuration is the hard cap on a single Music session.
// Once a session has been running this long it self-terminates
// even if the user is still looking at the dashboard. Keeps a
// forgotten chat from racking up 17 280 edits per day.
const DefaultMaxDuration = 10 * time.Minute

// Manager owns the per-chat live-refresh goroutines for the 🎵
// Music dashboard. One Manager per process; sessions are keyed
// by chat ID.
//
// Lifecycle:
//   - Constructed once at daemon startup via [NewManager],
//     stored on bot.Deps.MusicRefresh.
//   - The bot pointer is wired later via [Manager.SetBot] (the
//     daemon's pingOnBoot goroutine waits for bot.Bot to be set
//     by [tgbot.Bot.Start]).
//   - [Manager.Start] is called from handleMusicOpen after the
//     initial sendPhoto succeeds.
//   - [Manager.Stop] is called from the callback router whenever
//     a non-music callback fires for a chat that has an active
//     session.
//
// Concurrency:
//   - Public methods serialise on mu. Per-session goroutines
//     own their own state (msgID, trackID, deadline) and only
//     interact with the Manager through the cancel func they
//     received at Start time.
//
// Field roles:
//   - mu serialises every access to active.
//   - active maps chat ID to its current session.
//   - music / sound are the read-side data sources for tick
//     re-renders.
//   - tick / max are the configurable cadence + cap; defaulted
//     to [DefaultTickInterval] / [DefaultMaxDuration] in
//     production but overridable for tests.
//   - bot is the Telegram client; ticks call EditMessageCaption
//     / EditMessageMedia through it. Read under mu so SetBot
//     can swap it in race-free.
//   - log captures tick errors at WARN — a failing edit (e.g.
//     Telegram 429 throttle) shouldn't kill the session, just
//     skip a tick.
type Manager struct {
	mu     sync.Mutex
	active map[int64]*session

	musicSvc *music.Service
	soundSvc *sound.Service
	bot      *tgbot.Bot
	log      *slog.Logger

	tick time.Duration
	max  time.Duration
}

// session is one chat's active refresher. Owned end-to-end by
// the goroutine spawned in [Manager.Start]; the Manager only
// holds a cancel func + bookkeeping for [Manager.Stop] /
// [Manager.Touch].
type session struct {
	cancel   context.CancelFunc
	msgID    int
	trackID  string
	deadline time.Time

	// touched is set by Manager.Touch to push freshly-fetched
	// state into the next tick without spawning a redundant
	// nowplaying-cli call. Nil when no fresh state is queued.
	touched *touched
}

// touched is the queued-state payload [Manager.Touch] hands to
// the goroutine. The goroutine consumes it on the next tick and
// clears the slot.
type touched struct {
	np  music.NowPlaying
	vol sound.State
}

// NewManager returns a [Manager] with default tick + cap. The
// bot pointer must be wired via [Manager.SetBot] before any
// session starts editing; that's normally done by the daemon
// after [tgbot.Bot.Start] populates bot.Deps.Bot.
//
// Pass [music.New] / [sound.New] in production; pass services
// backed by [runner.NewFake] in tests.
func NewManager(musicSvc *music.Service, soundSvc *sound.Service, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		active:   map[int64]*session{},
		musicSvc: musicSvc,
		soundSvc: soundSvc,
		log:      log,
		tick:     DefaultTickInterval,
		max:      DefaultMaxDuration,
	}
}

// SetBot wires the Telegram client. Called from the daemon
// after bot.Start populates bot.Deps.Bot. Safe to call
// multiple times (overwrites the previous pointer).
func (m *Manager) SetBot(b *tgbot.Bot) {
	m.mu.Lock()
	m.bot = b
	m.mu.Unlock()
}

// SetTick overrides [DefaultTickInterval]. Used by tests for
// fast-iteration runs; production code should not call this.
func (m *Manager) SetTick(d time.Duration) {
	m.mu.Lock()
	m.tick = d
	m.mu.Unlock()
}

// SetMax overrides [DefaultMaxDuration]. Used by tests to
// exercise the hard-cap branch quickly; production code should
// not call this.
func (m *Manager) SetMax(d time.Duration) {
	m.mu.Lock()
	m.max = d
	m.mu.Unlock()
}

// Start spawns a refresher goroutine for chatID that will
// edit msgID's caption every tick interval until cancelled or
// the deadline elapses.
//
// Behavior:
//   - Cancels any pre-existing session for chatID before
//     installing the new one.
//   - The initial NowPlaying snapshot is used to seed
//     trackID-change detection on the first tick.
//   - The goroutine inherits a child context derived from
//     ctx; cancelling ctx (e.g. on daemon shutdown) cancels
//     every session.
//   - SetBot must have been called before Start; otherwise
//     ticks log "no bot wired" at WARN and skip the edit
//     (the session itself stays alive).
func (m *Manager) Start(ctx context.Context, chatID int64, msgID int, initial music.NowPlaying) {
	m.mu.Lock()
	if old, ok := m.active[chatID]; ok {
		old.cancel()
	}
	// gosec G118 can't see that cancel is stored in session.cancel
	// and invoked by Stop / the goroutine's defer. The lint is a
	// false positive here.
	sessCtx, cancel := context.WithCancel(ctx) //nolint:gosec // cancel stored in session.cancel; called by Stop and run's defer
	s := &session{
		cancel:   cancel,
		msgID:    msgID,
		trackID:  initial.TrackID,
		deadline: time.Now().Add(m.max),
	}
	m.active[chatID] = s
	tickInterval := m.tick
	m.mu.Unlock()

	go m.run(sessCtx, chatID, s, tickInterval)
}

// Stop cancels the active session for chatID. No-op when no
// session exists for that chat. Returns true when a session
// was actually present and cancelled.
func (m *Manager) Stop(chatID int64) bool {
	m.mu.Lock()
	s, ok := m.active[chatID]
	if ok {
		delete(m.active, chatID)
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	s.cancel()
	return true
}

// Touch pushes a fresh NowPlaying + sound.State into the
// session's next-tick slot, used by handlers that just changed
// state (play, pause, vol-up, …) so the user sees the change
// before the regular tick fires. No-op when no session exists.
func (m *Manager) Touch(chatID int64, np music.NowPlaying, vol sound.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.active[chatID]
	if !ok {
		return
	}
	s.touched = &touched{np: np, vol: vol}
}

// IsActive reports whether chatID has a running session. Used
// by tests + the dispatcher's debug paths.
func (m *Manager) IsActive(chatID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.active[chatID]
	return ok
}

// run is the per-chat goroutine. Polls + edits every tick
// until the context cancels OR the deadline elapses.
//
// Defers s.cancel() so the context derived from the parent
// is always released (Stop calls cancel from outside the
// goroutine; the defer makes the natural-exit path equally
// clean and silences gosec's G118 about an unused-looking
// cancel func).
func (m *Manager) run(ctx context.Context, chatID int64, s *session, interval time.Duration) {
	defer s.cancel()
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			m.evict(chatID)
			return
		case <-t.C:
			if time.Now().After(s.deadline) {
				m.log.Info("music refresh hit max duration; stopping",
					"chat_id", chatID, "max", m.max)
				m.evict(chatID)
				return
			}
			m.tickOnce(ctx, chatID, s)
		}
	}
}

// evict removes the session for chatID from the active map IF
// the entry still belongs to this goroutine (a Stop+Start race
// could have replaced it; we don't want to delete the new
// session). Cancels nothing (the caller is the goroutine that's
// about to return).
func (m *Manager) evict(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, chatID)
}

// tickOnce runs one refresh cycle: pops any queued Touch state,
// fetches fresh state otherwise, and emits an editMessageCaption
// (or editMessageMedia on track change).
func (m *Manager) tickOnce(ctx context.Context, chatID int64, s *session) {
	m.mu.Lock()
	bot := m.bot
	queued := s.touched
	s.touched = nil
	m.mu.Unlock()

	if bot == nil {
		m.log.Warn("music refresh: no bot wired; skipping tick", "chat_id", chatID)
		return
	}

	var np music.NowPlaying
	var vol sound.State
	var err error
	if queued != nil {
		np = queued.np
		vol = queued.vol
	} else {
		np, err = m.musicSvc.Get(ctx)
		if err != nil {
			m.log.Warn("music refresh: get failed; skipping tick",
				"chat_id", chatID, "err", err)
			return
		}
		vol, err = m.soundSvc.Get(ctx)
		if err != nil {
			m.log.Warn("music refresh: sound.Get failed; skipping volume update",
				"chat_id", chatID, "err", err)
		}
	}

	caption := keyboards.MusicCaption(np, vol, true)
	kb := keyboards.MusicKeyboard(np.IsPlaying(), vol.Muted, true)

	if np.TrackID != "" && np.TrackID != s.trackID {
		// Track change → swap the photo via editMessageMedia, fetching
		// the new artwork once.
		fresh, err := m.musicSvc.GetWithArtwork(ctx)
		if err == nil && fresh.TrackID == np.TrackID {
			art := fresh.Artwork
			if len(art) == 0 {
				art = Placeholder
			}
			photo := &models.InputMediaPhoto{
				Media:           "attach://artwork",
				Caption:         caption,
				ParseMode:       models.ParseModeMarkdown,
				MediaAttachment: bytes.NewReader(art),
			}
			if _, err := bot.EditMessageMedia(ctx, &tgbot.EditMessageMediaParams{
				ChatID:      chatID,
				MessageID:   s.msgID,
				Media:       photo,
				ReplyMarkup: kb,
			}); err != nil {
				m.log.Warn("music refresh: editMessageMedia failed",
					"chat_id", chatID, "err", err)
				return
			}
			s.trackID = np.TrackID
			return
		}
		// On artwork-fetch failure fall through to a caption-only edit
		// — better to keep the dashboard live than to abandon the tick.
	}

	if _, err := bot.EditMessageCaption(ctx, &tgbot.EditMessageCaptionParams{
		ChatID:      chatID,
		MessageID:   s.msgID,
		Caption:     caption,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	}); err != nil {
		m.log.Warn("music refresh: editMessageCaption failed",
			"chat_id", chatID, "err", err)
	}
}
