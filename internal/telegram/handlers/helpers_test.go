package handlers_test

import (
	"log/slog"
	"testing"
	"time"

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
	"github.com/amiwrpremium/macontrol/internal/runner"
	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
	"github.com/amiwrpremium/macontrol/internal/telegram/flows"
	"github.com/amiwrpremium/macontrol/internal/telegram/musicrefresh"
	"github.com/amiwrpremium/macontrol/internal/telegram/telegramtest"
)

// harness bundles a real-wired Deps with a Fake runner and an httptest-backed
// bot for handler tests.
type harness struct {
	t        *testing.T
	Deps     *bot.Deps
	Recorder *telegramtest.Recorder
	Fake     *runner.Fake
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	b, rec := telegramtest.NewBot(t)
	f := runner.NewFake()

	flowReg := flows.NewRegistry(5 * time.Minute)
	sm := callbacks.NewShortMap(time.Minute)
	musicSvc := music.New(f)
	soundSvc := sound.New(f)
	mr := musicrefresh.NewManager(musicSvc, soundSvc, slog.New(slog.DiscardHandler))
	mr.SetBot(b)
	// Tick interval well above the longest individual test
	// timeout, so background ticks never race with assertions.
	mr.SetTick(time.Hour)
	mr.SetMax(time.Hour)

	deps := &bot.Deps{
		Bot:       b,
		Logger:    slog.New(slog.DiscardHandler),
		Whitelist: bot.NewWhitelist([]int64{42}),
		Flows:     flowReg,
		FlowReg:   flowReg,
		ShortMap:  sm,
		Services: bot.Services{
			Sound:     soundSvc,
			Display:   display.New(f),
			Power:     power.New(f),
			Battery:   battery.New(f),
			WiFi:      wifi.New(f),
			Bluetooth: bluetooth.New(f),
			System:    system.New(f),
			Media:     media.New(f),
			Notify:    notify.New(f),
			Tools:     tools.New(f),
			Music:     musicSvc,
			Apps:      apps.New(f),
			Status:    status.New(f),
		},
		Capability: capability.Report{
			Version:  capability.ParseVersion("15.3"),
			Features: capability.Features{NetworkQuality: true, Shortcuts: true, WdutilInfo: true, NowPlaying: true},
		},
		MusicRefresh: mr,
	}
	return &harness{t: t, Deps: deps, Recorder: rec, Fake: f}
}

// newCallbackUpdate builds a CallbackQuery update with a Message attached so
// handlers can invoke Reply.Edit.
func newCallbackUpdate(id, data string) *models.Update {
	return &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   id,
			Data: data,
			From: models.User{ID: 42},
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   100,
					Chat: models.Chat{ID: 42, Type: "private"},
				},
			},
		},
	}
}

func newMessageUpdate(text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: text,
			From: &models.User{ID: 42},
			Chat: models.Chat{ID: 42, Type: "private"},
		},
	}
}
