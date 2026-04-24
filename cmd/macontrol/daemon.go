package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/amiwrpremium/macontrol/internal/capability"
	"github.com/amiwrpremium/macontrol/internal/config"
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
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

// runDaemon is the long-running entry point for `macontrol run`
// (and the implicit default when no subcommand is given). Wires
// up every dependency, attaches the SIGINT/SIGTERM handler, and
// blocks on the bot's long-poll loop until cancellation.
//
// Lifecycle (in order):
//  1. Load config from the Keychain via [config.Load]. On
//     failure, prints to stderr and exits 1 — the daemon
//     can't start without a token + whitelist.
//  2. Build the structured logger via [newLogger]; the format
//     is slog text. Log file rotation comes from lumberjack.
//  3. Install a [signal.NotifyContext] so the daemon shuts
//     down cleanly on Ctrl-C or `launchctl bootout`.
//  4. Detect the macOS feature set via [capability.Detect];
//     non-fatal — failures fall through with an empty feature
//     set and a WARN log.
//  5. Construct every domain [bot.Services] off the same
//     [runner.New] so they share the subprocess boundary +
//     timeout policy.
//  6. Construct the [callbacks.NewShortMap] (15-min TTL) for
//     callback-data overflow and start its janitor.
//  7. Construct the [flows.NewRegistry] (5-min inactivity TTL)
//     for active multi-step flows and start its janitor.
//  8. Wire all of the above + the [handlers] routers + the
//     [bot.NewWhitelist] into [bot.Deps].
//  9. Spawn [pingOnBoot] in a goroutine to send the boot
//     "I'm up" message to every whitelisted user once the
//     bot client is connected.
//  10. Call [bot.Start] which long-polls until ctx is
//     cancelled. Failure exits 1.
//
// Never returns under normal operation — Start blocks for the
// process lifetime.
func runDaemon(logLevel, logFile string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "macontrol: %v\n", err)
		os.Exit(1)
	}
	logger := newLogger(logLevel, logFile)
	logger.Info("macontrol starting",
		"version", version, "commit", commit,
		"log_level", logLevel, "log_file", logFile,
		"secrets", "keychain")

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	r := runner.New()

	rep, err := capability.Detect(ctx, r)
	if err != nil {
		logger.Warn("capability detect failed — assuming empty feature set", "err", err)
	}
	logger.Info("capabilities", "summary", rep.Summary())

	services := bot.Services{
		Sound:     sound.New(r),
		Display:   display.New(r),
		Power:     power.New(r),
		Battery:   battery.New(r),
		WiFi:      wifi.New(r),
		Bluetooth: bluetooth.New(r),
		System:    system.New(r),
		Media:     media.New(r),
		Notify:    notify.New(r),
		Tools:     tools.New(r),
		Status:    status.New(r),
	}

	shortmap := callbacks.NewShortMap(15 * time.Minute)
	shortmap.StartJanitor(ctx.Done())

	flowReg := flows.NewRegistry(5 * time.Minute)
	flowReg.StartJanitor(ctx)

	deps := &bot.Deps{
		Logger:     logger,
		Whitelist:  bot.NewWhitelist(cfg.AllowedUserIDs),
		Commands:   handlers.NewCommandRouter(),
		Calls:      handlers.NewCallbackRouter(),
		Flows:      flowReg,
		Services:   services,
		Capability: rep,
		ShortMap:   shortmap,
		FlowReg:    flowReg,
	}

	go pingOnBoot(ctx, deps)

	if err := bot.Start(ctx, cfg.TelegramBotToken, deps); err != nil {
		logger.Error("bot exited", "err", err)
		cancel()
		//nolint:gocritic // explicit cancel() above flushes the context before exit
		os.Exit(1)
	}
}

// pingOnBoot sends the "macontrol is up" message ([handlers.BootPing])
// to every whitelisted Telegram user once the bot client has
// connected.
//
// Behavior:
//  1. Polls d.Bot up to 50 × 100ms (5 seconds total) waiting
//     for [bot.Start] to set the field. Returns silently on
//     timeout — the daemon will still work for incoming
//     updates, just no boot ping.
//  2. Honours ctx.Done during the wait; returns immediately if
//     the daemon is shutting down before connect.
//  3. Composes the boot message via [handlers.BootPing] and
//     pipes through [bot.MDToHTML] for HTML parse mode.
//  4. For every whitelisted user ID, first calls
//     [handlers.ClearLegacyReplyKB] to evict any stale
//     ReplyKeyboard from pre-v0.1.4 clients, then sends the
//     boot ping. Failures are logged at WARN per user; one
//     bad chat doesn't stop the others.
//
// Runs as a goroutine spawned by [runDaemon]; not user-facing.
func pingOnBoot(ctx context.Context, d *bot.Deps) {
	// Wait a moment for bot.Start to set d.Bot.
	for i := 0; i < 50 && d.Bot == nil; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	if d.Bot == nil {
		return
	}
	text := bot.MDToHTML(handlers.BootPing(ctx, d))
	for _, uid := range d.Whitelist.Members() {
		// Best-effort clear of any stale reply keyboard left over from
		// pre-v0.1.4 clients. See handlers.ClearLegacyReplyKB.
		handlers.ClearLegacyReplyKB(ctx, d, uid)
		_, err := d.Bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    uid,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			d.Logger.Warn("boot ping failed", "uid", uid, "err", err)
		}
	}
}

// newLogger constructs the daemon's structured logger.
//
// Behavior:
//   - logLevel maps to one of [slog.LevelDebug] / Info / Warn /
//     Error. Anything unknown defaults to Info silently.
//   - logFile == "" → emit slog text to stderr (useful for
//     development and `launchctl debug` flows).
//   - logFile non-empty → emit through a [lumberjack.Logger]
//     with: 10 MB per file, 5 backups retained, 30-day max
//     age, gzip-compressed rotated files. The defaults match
//     macOS's general expectation for ~/Library/Logs files.
//
// Returns the built [slog.Logger]. Never returns nil.
func newLogger(logLevel, logFile string) *slog.Logger {
	level := slog.LevelInfo
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	if logFile == "" {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	}
	rot := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
	}
	return slog.New(slog.NewTextHandler(rot, &slog.HandlerOptions{Level: level}))
}
