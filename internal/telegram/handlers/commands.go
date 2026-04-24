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

// CommandRouter dispatches "/cmd" Telegram messages to the
// matching per-command handler. Implements [bot.Router] so the
// upstream dispatcher can install it on bot.Deps.Commands.
//
// Lifecycle:
//   - Constructed once at daemon startup via [NewCommandRouter]
//     and stored on bot.Deps.Commands.
//
// Concurrency:
//   - Stateless; every Handle call is independent.
type CommandRouter struct{}

// NewCommandRouter returns a fresh [CommandRouter] ready to be
// installed on bot.Deps.Commands.
func NewCommandRouter() *CommandRouter { return &CommandRouter{} }

// Handle is the [bot.Router] implementation for slash commands.
//
// Routing rules (parsed command — first match wins):
//  1. "/start" or "/menu"   → [cmdMenu] — sends the inline home
//     keyboard.
//  2. "/status"             → [cmdStatus] — sends the
//     dashboard text snapshot.
//  3. "/help"               → [cmdHelp] — prints the slash-
//     command quick reference.
//  4. "/cancel"             → [cmdCancel] — terminates any
//     active multi-step flow for the chat.
//  5. "/lock"               → [cmdLock] — locks the screen
//     via [power.Service.Lock].
//  6. "/screenshot"         → [cmdScreenshot] — captures a
//     silent full-screen screenshot via [media.Service.Screenshot].
//
// Unknown commands fall through silently and return nil. The
// bot is intentionally quiet on unknowns so it doesn't spam
// group chats where another bot owns the command.
//
// The dispatcher in [bot.Deps.dispatch] only routes here when
// the message text starts with "/" and the sender is whitelisted.
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

// parseCommand extracts the leading slash command from a
// Telegram message and the remaining args (if any).
//
// Behavior:
//   - Trims surrounding whitespace.
//   - Splits on the first space; everything before is the
//     command, everything after is the rest.
//   - Strips a "@botname" suffix if present (Telegram appends
//     it in group chats so the command is unambiguous).
//
// Returns ("/foo", "args") for "/foo@mybot args"; ("", "") for
// an empty input.
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

// cmdMenu sends the inline home keyboard via
// [keyboards.HomeInlineTitle] + [keyboards.InlineHome]. Wraps
// the text through [bot.MDToHTML] for HTML parse-mode rendering.
//
// Behavior:
//   - First clears any legacy ReplyKeyboard via
//     [ClearLegacyReplyKB] — defensive against users upgrading
//     from earlier macontrol versions where the bot used
//     ReplyKeyboards instead of inline ones.
//   - Sends the home grid as a fresh message (NOT an edit) so
//     the keyboard is always reachable from the chat.
func cmdMenu(ctx context.Context, d *bot.Deps, u *models.Update) error {
	ClearLegacyReplyKB(ctx, d, u.Message.Chat.ID)
	_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      u.Message.Chat.ID,
		Text:        bot.MDToHTML(keyboards.HomeInlineTitle),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.InlineHome(),
	})
	return err
}

// cmdStatus sends the dashboard text snapshot composed by
// [status.Service.Snapshot] and rendered via [renderStatus].
// The home keyboard is attached so the user can drill into any
// category from the same message.
//
// Behavior:
//   - Errors from [status.Service.Snapshot] are swallowed —
//     [renderStatus] handles partial snapshots gracefully and
//     a "no info" reply is worse than an empty one.
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

// renderStatus composes the /status dashboard text from a
// [status.Snapshot]. Uses Markdown-style markers; callers must
// pipe the result through [bot.MDToHTML] before sending to
// Telegram's HTML parse mode.
//
// Behavior:
//   - Skips the OS line entirely when InfoErr is non-nil.
//   - Skips the battery line when BatteryErr is non-nil OR
//     when Battery.Present is false (desktop Macs).
//   - Skips the Wi-Fi line when WiFiErr is non-nil; otherwise
//     renders SSID with "—" when associated-but-empty.
//   - Skips the uptime line when InfoErr is non-nil; falls
//     back to Uptime.Raw when the parsed Duration is empty.
//
// Reused by [BootPing] for the daemon's startup ping.
func renderStatus(s status.Snapshot) string {
	var b strings.Builder
	b.WriteString("🖥 *macontrol status*\n\n")
	writeStatusInfoLine(&b, s)
	writeStatusBatteryLine(&b, s)
	writeStatusWiFiLine(&b, s)
	writeStatusUptimeLine(&b, s)
	return b.String()
}

func writeStatusInfoLine(b *strings.Builder, s status.Snapshot) {
	if s.InfoErr != nil {
		return
	}
	fmt.Fprintf(b, "• %s %s on `%s` (`%s`)\n", s.Info.ProductName, s.Info.ProductVersion, s.Info.Model, s.Info.Hostname)
}

func writeStatusBatteryLine(b *strings.Builder, s status.Snapshot) {
	if s.BatteryErr != nil || !s.Battery.Present {
		return
	}
	fmt.Fprintf(b, "• 🔋 %d%% · %s\n", s.Battery.Percent, s.Battery.State)
}

func writeStatusWiFiLine(b *strings.Builder, s status.Snapshot) {
	if s.WiFiErr != nil {
		return
	}
	ssid := s.WiFi.SSID
	if ssid == "" {
		ssid = "—"
	}
	power := "off"
	if s.WiFi.PowerOn {
		power = "on"
	}
	fmt.Fprintf(b, "• 📶 Wi-Fi `%s` · SSID `%s`\n", power, ssid)
}

func writeStatusUptimeLine(b *strings.Builder, s status.Snapshot) {
	if s.InfoErr != nil {
		return
	}
	if d := s.Info.Uptime.Duration; d != "" {
		fmt.Fprintf(b, "• ⏱ up `%s`\n", d)
		return
	}
	if s.Info.Uptime.Raw != "" {
		fmt.Fprintf(b, "• ⏱ %s\n", s.Info.Uptime.Raw)
	}
}

// BootPing returns the text the daemon sends to every
// whitelisted user when the process starts. Composes a
// "✅ macontrol is up" header with the current
// [status.Snapshot] (via [renderStatus]) and an italic
// capability summary footer.
//
// Exposed (vs internal helper) so cmd/macontrol/daemon.go can
// call it from main without needing an exported handler. The
// daemon sends the result through [bot.MDToHTML] +
// [models.ParseModeHTML] same as the regular /status path.
func BootPing(ctx context.Context, d *bot.Deps) string {
	snap, _ := d.Services.Status.Snapshot(ctx)
	return "✅ *macontrol is up*\n\n" + renderStatus(snap) +
		"\n_" + d.Capability.Summary() + "_"
}

// _ = battery.StateCharging keeps the [battery] import alive
// even though no symbol is referenced after the recent
// refactors. Removing the alias would break the import; keeping
// it lets `go doc` still show the package as a transitive
// dependency. See the smells list — better fixed by dropping
// the import.
var _ = battery.StateCharging

// cmdHelp sends the slash-command quick reference.
//
// Behavior:
//   - Static text composed inline with backtick-wrapped command
//     names so MDToHTML renders them as `<code>` spans.
//   - Sent without a keyboard — the help text already lists
//     `/menu` as the entry point.
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

// cmdCancel terminates the active flow (if any) for the chat.
//
// Behavior:
//   - Calls [bot.FlowManager.Cancel] which returns true when
//     a flow was actually present and cleared, false on no-op.
//   - Sends "✖ flow cancelled." on hit, "🧹 nothing to cancel."
//     on miss — distinct messages so the user can tell whether
//     /cancel did anything.
//
// Note: the dispatcher in [bot.Deps.dispatch] does NOT route
// /cancel to the active flow's Handle; this command is
// intercepted at the slash-command layer because it must work
// regardless of what the flow's parser would do with the text.
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

// cmdLock locks the screen via [power.Service.Lock] (which
// uses `pmset displaysleepnow` — see the [power] package for
// the lock-vs-display-sleep nuance).
//
// Behavior:
//   - On lock failure, sends a "⚠ lock failed: <err>" message
//     and returns the error.
//   - On success, sends "🔒 locked.".
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

// cmdScreenshot captures a silent full-screen screenshot via
// [media.Service.Screenshot] and uploads it via [Reply.SendPhoto].
//
// Behavior:
//   - Uses [mediaSilentOpts] (defined in handlers/med.go) for
//     the "every display, no shutter sound" capture.
//   - On capture failure, sends a "⚠ screenshot failed: <err>"
//     message and returns the error. The temp file is
//     guaranteed cleaned up by Service.Screenshot itself on
//     failure, and by Reply.SendPhoto on success.
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
