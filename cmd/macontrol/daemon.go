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

// pingOnBoot sends the "I'm up" message to every whitelisted user once the
// bot client is initialised.
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
