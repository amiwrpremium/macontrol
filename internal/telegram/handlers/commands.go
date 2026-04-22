package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/domain/battery"
	"github.com/amiwrpremium/macontrol/internal/domain/status"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// CommandRouter implements bot.Router for `/…` slash commands.
type CommandRouter struct{}

// NewCommandRouter returns a Router.
func NewCommandRouter() *CommandRouter { return &CommandRouter{} }

// Handle implements bot.Router.
func (CommandRouter) Handle(ctx context.Context, d *bot.Deps, update *models.Update) error {
	cmd, _ := parseCommand(update.Message.Text)
	switch cmd {
	case "/start", "/menu":
		return cmdMenu(ctx, d, update)
	case "/status":
		return cmdStatus(ctx, d, update)
	case "/help":
		return cmdHelp(ctx, d, update)
	case "/cancel":
		return cmdCancel(ctx, d, update)
	case "/lock":
		return cmdLock(ctx, d, update)
	case "/screenshot":
		return cmdScreenshot(ctx, d, update)
	}
	// Unknown command — fall through silently; avoid echoing in groups.
	return nil
}

func parseCommand(text string) (cmd string, rest string) {
	text = strings.TrimSpace(text)
	parts := strings.SplitN(text, " ", 2)
	cmd = parts[0]
	if at := strings.Index(cmd, "@"); at > 0 {
		cmd = cmd[:at]
	}
	if len(parts) == 2 {
		rest = parts[1]
	}
	return
}

func cmdMenu(ctx context.Context, d *bot.Deps, u *models.Update) error {
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      u.Message.Chat.ID,
		Text:        bot.MDToHTML(keyboards.HomeInlineTitle),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.InlineHome(),
	})
	return err
}

func cmdStatus(ctx context.Context, d *bot.Deps, u *models.Update) error {
	snap, _ := d.Services.Status.Snapshot(ctx)
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      u.Message.Chat.ID,
		Text:        bot.MDToHTML(renderStatus(snap)),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.InlineHome(),
	})
	return err
}

// renderStatus composes the /status dashboard using Markdown-style
// markers; callers pipe the result through bot.MDToHTML before sending
// to Telegram's HTML parse mode. Also reused by BootPing.
func renderStatus(s status.Snapshot) string {
	var b strings.Builder
	b.WriteString("🖥 *macontrol status*\n\n")
	if s.InfoErr == nil {
		fmt.Fprintf(&b, "• %s %s on `%s` (`%s`)\n", s.Info.ProductName, s.Info.ProductVersion, s.Info.Model, s.Info.Hostname)
	}
	if s.BatteryErr == nil && s.Battery.Present {
		fmt.Fprintf(&b, "• 🔋 %d%% · %s\n", s.Battery.Percent, s.Battery.State)
	}
	if s.WiFiErr == nil {
		ssid := s.WiFi.SSID
		if ssid == "" {
			ssid = "—"
		}
		power := "off"
		if s.WiFi.PowerOn {
			power = "on"
		}
		fmt.Fprintf(&b, "• 📶 Wi-Fi `%s` · SSID `%s`\n", power, ssid)
	}
	if s.InfoErr == nil {
		if d := s.Info.Uptime.Duration; d != "" {
			fmt.Fprintf(&b, "• ⏱ up `%s`\n", d)
		} else if s.Info.Uptime.Raw != "" {
			fmt.Fprintf(&b, "• ⏱ %s\n", s.Info.Uptime.Raw)
		}
	}
	return b.String()
}

// BootPing returns the text sent to every whitelisted user when the
// daemon starts. Exposed so main.go can call it. The daemon sends it
// through bot.MDToHTML + ParseModeHTML.
func BootPing(ctx context.Context, d *bot.Deps) string {
	snap, _ := d.Services.Status.Snapshot(ctx)
	return "✅ *macontrol is up*\n\n" + renderStatus(snap) +
		"\n_" + d.Capability.Summary() + "_"
}

// ignore unused battery import safety
var _ = battery.StateCharging

func cmdHelp(ctx context.Context, d *bot.Deps, u *models.Update) error {
	text := `🤖 *macontrol*

The menu-first way to control your Mac. Use *` + "`/menu`" + `* to summon the home keyboard.

*Slash commands:*
• ` + "`/menu`" + ` — show the home keyboard
• ` + "`/status`" + ` — dashboard text snapshot
• ` + "`/lock`" + ` — lock the screen immediately
• ` + "`/screenshot`" + ` — full-screen silent screenshot
• ` + "`/cancel`" + ` — cancel an active multi-step flow
• ` + "`/help`" + ` — this message

Full docs: github.com/amiwrpremium/macontrol`
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    u.Message.Chat.ID,
		Text:      bot.MDToHTML(text),
		ParseMode: models.ParseModeHTML,
	})
	return err
}

func cmdCancel(ctx context.Context, d *bot.Deps, u *models.Update) error {
	chatID := u.Message.Chat.ID
	cancelled := d.Flows.Cancel(chatID)
	text := "🧹 nothing to cancel."
	if cancelled {
		text = "✖ flow cancelled."
	}
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func cmdLock(ctx context.Context, d *bot.Deps, u *models.Update) error {
	if err := d.Services.Power.Lock(ctx); err != nil {
		_, _ = d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    u.Message.Chat.ID,
			Text:      bot.MDToHTML("⚠ lock failed: `" + err.Error() + "`"),
			ParseMode: models.ParseModeHTML,
		})
		return err
	}
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: u.Message.Chat.ID,
		Text:   "🔒 locked.",
	})
	return err
}

func cmdScreenshot(ctx context.Context, d *bot.Deps, u *models.Update) error {
	chatID := u.Message.Chat.ID
	svc := d.Services.Media
	path, err := svc.Screenshot(ctx, mediaSilentOpts())
	if err != nil {
		_, _ = d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    chatID,
			Text:      bot.MDToHTML("⚠ screenshot failed: `" + err.Error() + "`"),
			ParseMode: models.ParseModeHTML,
		})
		return err
	}
	return Reply{Deps: d}.SendPhoto(ctx, chatID, path, "📷 screenshot")
}
