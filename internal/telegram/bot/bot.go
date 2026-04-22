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
	Sound     *sound.Service
	Display   *display.Service
	Power     *power.Service
	Battery   *battery.Service
	WiFi      *wifi.Service
	Bluetooth *bluetooth.Service
	System    *system.Service
	Media     *media.Service
	Notify    *notify.Service
	Tools     *tools.Service
	Status    *status.Service
}

// Deps bundles everything a handler may need: the underlying bot client
// (for replies, edits, answerCallbackQuery), the logger, and the capability
// report gathered at startup.
type Deps struct {
	Bot        *tgbot.Bot
	Logger     *slog.Logger
	Whitelist  Whitelist
	Commands   Router
	Calls      Router
	Flows      FlowManager
	Services   Services
	Capability capability.Report
	ShortMap   *callbacks.ShortMap
	FlowReg    *flows.Registry
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
