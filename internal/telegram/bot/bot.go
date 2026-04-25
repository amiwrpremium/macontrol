// Package bot wires the upstream go-telegram client to macontrol's
// own routers, middleware, and per-domain services.
//
// The package is the composition root for the Telegram side of the
// daemon: [Deps] holds every cross-cutting dependency a handler may
// need (the bot client, the logger, the whitelist, the dispatchers,
// the [Services] bundle, the boot-time capability report), and
// [Start] connects, registers a single root handler, and blocks
// until ctx is cancelled.
//
// The dispatch flow is intentionally narrow:
//
//  1. recover any panic in the handler tree.
//  2. drop any update whose sender is not in the [Whitelist].
//  3. fan out by update kind:
//     callback_query → [Deps.Calls],
//     /-prefixed text message → [Deps.Commands],
//     plain text → active [flows.Flow] for the chat (if any),
//     anything else → silently ignored.
//
// Anything more elaborate (pagination, multi-step flows, stateful
// drill-downs) lives downstream in handlers/, keyboards/, and
// flows/.
package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/apps"
	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/domain/media"
	"github.com/amiwrpremium/macontrol/internal/domain/music"
	"github.com/amiwrpremium/macontrol/internal/domain/notify"
	"github.com/amiwrpremium/macontrol/internal/domain/power"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/domain/status"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/musicrefresh"
)

// Services bundles every domain Service the bot may call into. One
// field per dashboard category; handlers and flows reach into this
// struct to invoke the underlying macOS-CLI orchestration.
//
// Lifecycle:
//   - Constructed once at daemon startup (cmd/macontrol/daemon.go),
//     each service receiving the same [runner.Runner] so they
//     share the subprocess boundary and timeout policy.
//   - All fields are immutable after startup; handlers treat the
//     struct as a read-only catalog.
//
// Concurrency:
//   - Each underlying *Service is safe for concurrent use (they
//     each hold a Runner, which is itself concurrent-safe). Reading
//     fields off Services from multiple goroutines is safe; the
//     struct is never mutated post-construction.
//
// Field roles:
//   - The first ten fields each map 1-to-1 to a dashboard category
//     namespace defined in [callbacks].
//   - [Services.Status] is the cross-cutting aggregator that reads
//     the others to compose the "/dash" snapshot; it does not own
//     a CLI of its own.
type Services struct {
	// Sound drives volume +/-, mute toggle, and the say-via-TTS
	// flow. Backed by `osascript` and `say`.
	Sound *sound.Service

	// Display drives brightness +/- and the screensaver trigger.
	// Requires the `brightness` brew formula for level read/set.
	Display *display.Service

	// Power drives sleep, lock, restart, shutdown, logout, and
	// caffeinate. Restart/shutdown/logout require the narrow
	// sudoers entry for `pmset` and `shutdown`.
	Power *power.Service

	// Battery reads charge level + state + health (cycle count,
	// condition, max capacity). Returns degraded state on
	// desktop Macs without a battery.
	Battery *battery.Service

	// WiFi controls the Wi-Fi radio (toggle), reads SSID + rich
	// link details, joins networks, sets DNS presets, and runs
	// the networkQuality speed test.
	WiFi *wifi.Service

	// Bluetooth controls the Bluetooth radio and lists/connects
	// paired devices via the `blueutil` brew formula.
	Bluetooth *bluetooth.Service

	// System reads OS + hardware info, thermal pressure, memory
	// stats + top processes, CPU stats + top processes, and
	// supports kill / force-kill of any pid.
	System *system.Service

	// Media captures full-screen and per-display screenshots,
	// records the screen for N seconds, and takes single-frame
	// webcam photos via the `imagesnap` brew formula.
	Media *media.Service

	// Notify sends desktop notifications (terminal-notifier
	// preferred, osascript fallback) and uses `say` for
	// text-to-speech.
	Notify *notify.Service

	// Tools groups clipboard get/set, the IANA timezone picker,
	// `sntp` time-sync, the disks list with per-disk drill-down,
	// and the Shortcuts.app runner.
	Tools *tools.Service

	// Music drives now-playing metadata, playback control, and
	// seek for any macOS player that publishes Now Playing info
	// (Music.app, Spotify, Podcasts, browser audio). Backed by
	// the third-party `nowplaying-cli` brew formula; gated on
	// [capability.Features.NowPlaying].
	Music *music.Service

	// Apps lists running user-facing applications and exposes
	// per-app Quit (graceful), ForceQuit (SIGKILL), and Hide
	// (Cmd-H) verbs, plus the multi-select "quit all except…"
	// bulk-quit. Backed by `osascript` + `kill`; requires
	// Accessibility TCC.
	Apps *apps.Service

	// Status composes a single "everything at a glance" snapshot
	// by reading from the other services; backs the legacy
	// /dash command.
	Status *status.Service
}

// Deps is the cross-cutting dependency bag passed to every router
// and handler. One instance per process, constructed at daemon
// startup, mostly immutable thereafter.
//
// Lifecycle:
//   - Built field-by-field in cmd/macontrol/daemon.go before
//     [Start] is called.
//   - [Deps.Bot] is set by [Start] on connect (and refreshed in
//     every dispatch as a defensive belt-and-suspenders); every
//     other field is set at construction and never mutated.
//   - Handlers and flows treat the value as a read-only catalog.
//
// Concurrency:
//   - Read access is safe from any goroutine (the underlying
//     services and registries are themselves concurrent-safe;
//     [Deps.Bot] is set at most twice per dispatch and the upstream
//     library treats it as immutable per call).
//   - The mutable-looking fields (Bot reassignment in [Deps.dispatch])
//     are technically a data race in pathological scheduling but
//     harmless because they always assign the same pointer the
//     library handed back at Start; flag in the smells list.
//
// Field roles:
//
//   - Bot is the upstream client. Populated by [Start]; handlers
//     may assume non-nil.
//
//   - Logger is the structured slog handle. Never nil; panic-on-nil
//     is acceptable.
//
//   - Whitelist is the auth boundary; consulted by [Deps.dispatch]
//     before any handler runs.
//
//   - Commands and Calls are the two top-level routers ([Router]
//     interface). Tests substitute fake routers via the
//     handlers/handler_test.go harness.
//
//   - Flows is the narrow [FlowManager] view used by the
//     dispatcher; FlowReg is the concrete *flows.Registry exposed
//     for handlers that need Install/Cancel outside the dispatch
//     path.
//
//   - Services holds the per-domain Service catalog (see [Services]).
//
//   - Capability is the boot-time macOS feature-gate report.
//
//   - ShortMap holds callback-data overflow values keyed by short
//     opaque ids.
type Deps struct {
	// Bot is the upstream go-telegram client. Populated by
	// [Start] on connect and by [Deps.dispatch] as a defensive
	// re-assignment; handlers may assume non-nil after the first
	// dispatch.
	Bot *tgbot.Bot

	// Logger is the structured logger; never nil.
	Logger *slog.Logger

	// Whitelist enforces the "is this user allowed" gate on every
	// incoming update. See auth.go for the membership check.
	Whitelist Whitelist

	// Commands dispatches "/…" slash-command messages to the
	// per-command handler.
	Commands Router

	// Calls dispatches inline-keyboard callback_query updates to
	// the per-namespace handler.
	Calls Router

	// Flows is the narrow [FlowManager] view used by the root
	// dispatcher; see also FlowReg for the concrete registry.
	Flows FlowManager

	// Services holds the per-domain Service instances used by
	// handlers and flows.
	Services Services

	// Capability is the boot-time macOS feature-gate report
	// (Wi-Fi info available, networkQuality available, Shortcuts
	// CLI available, …).
	Capability capability.Report

	// ShortMap stores callback-data overflow values (long disk
	// paths, full timezone names, Shortcuts titles) keyed by
	// short opaque ids that fit inside the 64-byte
	// callback_data budget.
	ShortMap *callbacks.ShortMap

	// FlowReg is the concrete flow registry; exposed for command
	// handlers that need to Install or Cancel a flow outside the
	// dispatch path.
	FlowReg *flows.Registry

	// MusicRefresh owns the per-chat live-refresh goroutines for
	// the 🎵 Music dashboard. Started by the music handler when
	// the user opens Music; stopped by the callback router when
	// the user navigates away. Set to a zero-value *Manager when
	// the daemon is constructed and wired with the bot pointer
	// after [tgbot.Bot.Start] runs.
	MusicRefresh *musicrefresh.Manager
}

// Router dispatches one kind of [models.Update] to the matching
// handler.
//
// Behavior:
//   - Implementations consume an update they understand
//     (callback_query for [Deps.Calls], "/cmd" message for
//     [Deps.Commands]) and return any handler error.
//   - Errors are logged at WARN by [Deps.dispatch] and otherwise
//     swallowed — Telegram retries are not appropriate for most
//     bot operations.
type Router interface {
	// Handle processes update on behalf of d. Returns nil on
	// success or any handler-level error to be logged by the
	// caller.
	Handle(ctx context.Context, d *Deps, update *models.Update) error
}

// FlowManager is the narrow read-and-mutate slice of
// *flows.Registry that the bot's dispatcher needs. Implemented
// by *flows.Registry; tests substitute their own fakes.
//
// Lifecycle:
//   - One implementation per process, owned by the registry on
//     [Deps.FlowReg].
//   - Active is consulted on every plain-text message;
//     Install/Cancel/Finish are called by handlers and flows
//     themselves to manage state.
type FlowManager interface {
	// Active returns the current flow for chatID, if any.
	// (nil, false) means no flow is in progress for that chat.
	Active(chatID int64) (flows.Flow, bool)

	// Cancel removes any in-progress flow for chatID and returns
	// true when a flow was actually cancelled.
	Cancel(chatID int64) bool

	// Install registers f as the active flow for chatID,
	// replacing any prior flow.
	Install(chatID int64, f flows.Flow)

	// Finish removes the active flow for chatID. No-op if none
	// is registered.
	Finish(chatID int64)
}

// Start connects the bot to Telegram and blocks until ctx is
// cancelled. token is the @BotFather token previously stored in
// the macOS Keychain.
//
// Behavior:
//   - Constructs the upstream client with two handlers attached:
//     the root [Deps.dispatch] for every update kind, and an
//     errors handler that logs transport-level errors at ERROR.
//   - Stashes the constructed client on d.Bot so handlers can
//     reach it.
//   - Logs "bot started" at INFO.
//   - Calls b.Start(ctx) which long-polls until ctx is cancelled.
//
// Returns the wrapped construction error if tgbot.New rejects the
// token; never returns an error from b.Start (the upstream library
// loops internally and reports transport errors via the
// WithErrorsHandler hook).
func Start(ctx context.Context, token string, d *Deps) error {
	opts := []tgbot.Option{
		tgbot.WithDefaultHandler(d.dispatch),
		tgbot.WithErrorsHandler(func(err error) {
			d.Logger.Error("telegram transport error", "err", err)
		}),
	}
	b, err := tgbot.New(token, opts...)
	if err != nil {
		return fmt.Errorf("bot.New: %w", err)
	}
	d.Bot = b

	d.Logger.Info("bot started")
	b.Start(ctx)
	return nil
}

// dispatch is the root update handler registered with the upstream
// bot client. Every Telegram update lands here.
//
// Behavior:
//  1. Defers a recover() that logs any panic at ERROR with the
//     panic value attached. Never re-panics.
//  2. Refreshes d.Bot from the per-call b argument (defensive —
//     [Start] already set it once).
//  3. Calls d.Whitelist.Allows; on reject, logs at WARN with the
//     sender id and returns silently.
//
// Routing rules (first match wins):
//  1. update.CallbackQuery non-nil → [Deps.Calls].Handle. Errors
//     logged at WARN; not returned.
//  2. update.Message non-nil with non-empty Text and a leading "/"
//     → [Deps.Commands].Handle. Errors logged at WARN.
//  3. update.Message non-nil with non-empty Text (no leading "/")
//     → [Deps.dispatchFlow]; either consumed by the active flow
//     or silently logged at DEBUG and dropped.
//  4. Anything else (edited messages, channel posts, polls, …)
//     → silently dropped. The bot intentionally ignores update
//     kinds it doesn't understand.
func (d *Deps) dispatch(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	defer func() {
		if r := recover(); r != nil {
			d.Logger.Error("panic in handler", "panic", fmt.Sprintf("%v", r))
		}
	}()
	d.Bot = b

	if !d.Whitelist.Allows(update) {
		d.Logger.Warn("rejected update from non-whitelisted user",
			"sender", senderID(update))
		return
	}

	switch {
	case update.CallbackQuery != nil:
		d.Logger.Debug("callback", "data", update.CallbackQuery.Data, "from", update.CallbackQuery.From.ID)
		if err := d.Calls.Handle(ctx, d, update); err != nil {
			d.Logger.Warn("callback dispatch", "err", err)
		}

	case update.Message != nil && update.Message.Text != "":
		if isCommand(update.Message.Text) {
			d.Logger.Debug("command", "text", update.Message.Text, "from", update.Message.From.ID)
			if err := d.Commands.Handle(ctx, d, update); err != nil {
				d.Logger.Warn("command dispatch", "err", err)
			}
			return
		}
		// Non-command text: maybe a flow is consuming it.
		d.dispatchFlow(ctx, update)
	}
}

// dispatchFlow routes a plain-text (non-command) message to the
// active [flows.Flow] for the chat, if one is registered.
//
// Behavior:
//   - Looks up the active flow via [Deps.Flows.Active]. Missing
//     flow → log at DEBUG and return silently.
//   - Calls flow.Handle(ctx, text). On (resp.Done == true), unwires
//     the flow via [FlowManager.Finish].
//   - On (resp.Text != ""), sends the response back to the chat.
//     ParseMode defaults to HTML, with [MDToHTML] used to convert
//     any Markdown-ish markup the flow emitted.
//   - Send errors are logged at WARN with the flow name; not
//     returned.
func (d *Deps) dispatchFlow(ctx context.Context, update *models.Update) {
	chatID := update.Message.Chat.ID
	flow, ok := d.Flows.Active(chatID)
	if !ok {
		d.Logger.Debug("ignored plain text (no active flow)", "from", update.Message.From.ID)
		return
	}
	resp := flow.Handle(ctx, update.Message.Text)
	if resp.Done {
		d.Flows.Finish(chatID)
	}
	if resp.Text != "" {
		parseMode := resp.ParseMode
		text := resp.Text
		if parseMode == "" {
			parseMode = models.ParseModeHTML
			text = MDToHTML(text)
		}
		_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ParseMode:   parseMode,
			ReplyMarkup: resp.Markup,
		})
		if err != nil {
			d.Logger.Warn("flow reply", "err", err, "flow", flow.Name())
		}
	}
}

// isCommand reports whether text is a Telegram command — i.e. has
// at least one character beyond the leading "/". A bare "/" does
// not count (no command name follows).
func isCommand(text string) bool {
	return len(text) > 1 && text[0] == '/'
}
