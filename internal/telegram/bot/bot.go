// Package bot wires the go-telegram/bot client to macontrol's routers and
// middleware stack.
package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/bluetooth"
	"github.com/amiwrpremium/macontrol/internal/domain/display"
	"github.com/amiwrpremium/macontrol/internal/domain/media"
	"github.com/amiwrpremium/macontrol/internal/domain/notify"
	"github.com/amiwrpremium/macontrol/internal/domain/power"
	"github.com/amiwrpremium/macontrol/internal/domain/sound"
	"github.com/amiwrpremium/macontrol/internal/domain/status"
	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/domain/wifi"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
)

// Services bundles every domain Service the bot may call into.
type Services struct {
	// Sound drives volume, mute, and say.
	Sound *sound.Service
	// Display drives brightness.
	Display *display.Service
	// Power drives sleep, lock, restart, shutdown, caffeinate.
	Power *power.Service
	// Battery reads charge state and long-term health.
	Battery *battery.Service
	// WiFi controls the Wi-Fi radio, join, and speedtest.
	WiFi *wifi.Service
	// Bluetooth controls the Bluetooth radio and device pairing
	// via `blueutil`.
	Bluetooth *bluetooth.Service
	// System reads OS/hardware info, memory, CPU, processes.
	System *system.Service
	// Media captures screenshots, screen recordings, and webcam
	// photos.
	Media *media.Service
	// Notify sends desktop notifications and uses `say` for
	// text-to-speech.
	Notify *notify.Service
	// Tools groups clipboard, timezone, disks, and Shortcuts.
	Tools *tools.Service
	// Status composes the dashboard snapshot.
	Status *status.Service
}

// Deps bundles everything a handler may need: the underlying bot client
// (for replies, edits, answerCallbackQuery), the logger, and the capability
// report gathered at startup.
type Deps struct {
	// Bot is the go-telegram client; populated by [Start] on
	// connect. Handlers may assume non-nil.
	Bot *tgbot.Bot
	// Logger is the structured logger; never nil.
	Logger *slog.Logger
	// Whitelist enforces the "is this user allowed" gate on every
	// incoming update.
	Whitelist Whitelist
	// Commands dispatches `/…` slash-command messages.
	Commands Router
	// Calls dispatches inline-keyboard callback queries.
	Calls Router
	// Flows is the narrow [FlowManager] interface used by the
	// dispatcher; FlowReg holds the full registry for callers that
	// need Install/Cancel outside the dispatch path.
	Flows FlowManager
	// Services holds the per-domain Service instances used by
	// handlers and flows.
	Services Services
	// Capability is the boot-time detection result; handlers use
	// it to hide features that require a newer macOS.
	Capability capability.Report
	// ShortMap stores callback-data overflow values keyed by short
	// opaque ids.
	ShortMap *callbacks.ShortMap
	// FlowReg is the concrete flow registry; exposed so command
	// handlers can Install / Cancel flows directly.
	FlowReg *flows.Registry
}

// Router dispatches one kind of Update to the right handler.
type Router interface {
	Handle(ctx context.Context, d *Deps, update *models.Update) error
}

// FlowManager is implemented by *flows.Registry. It's the subset of the
// registry the bot's dispatcher needs to route plain-text messages to a
// live flow.
type FlowManager interface {
	Active(chatID int64) (flows.Flow, bool)
	Cancel(chatID int64) bool
	Install(chatID int64, f flows.Flow)
	Finish(chatID int64)
}

// Start runs the bot until ctx is cancelled. token is the @BotFather token.
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

// dispatch is the root handler. Order: recover → auth (done at router level
// via shouldAccept) → log → fan-out to callback/command/flow routers.
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

func isCommand(text string) bool {
	return len(text) > 1 && text[0] == '/'
}
