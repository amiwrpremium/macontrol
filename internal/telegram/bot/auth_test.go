package bot_test

import (
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/amiwrpremium/macontrol/internal/telegram/bot"
)

func TestNewWhitelist_EmptyIDs(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist(nil)
	if len(w.Members()) != 0 {
		t.Fatalf("expected empty, got %v", w.Members())
	}
}

func TestNewWhitelist_DeduplicatesInMap(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{1, 2, 2, 3, 1})
	if len(w.Members()) != 3 {
		t.Fatalf("expected 3 unique, got %v", w.Members())
	}
}

func TestAllows_Message(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{42})
	u := &models.Update{Message: &models.Message{From: &models.User{ID: 42}}}
	if !w.Allows(u) {
		t.Error("expected allowed")
	}
}

func TestAllows_MessageNotWhitelisted(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{42})
	u := &models.Update{Message: &models.Message{From: &models.User{ID: 999}}}
	if w.Allows(u) {
		t.Error("expected rejected")
	}
}

func TestAllows_CallbackQuery(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{42})
	u := &models.Update{CallbackQuery: &models.CallbackQuery{From: models.User{ID: 42}}}
	if !w.Allows(u) {
		t.Error("expected allowed")
	}
}

func TestAllows_MissingFrom(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{42})
	u := &models.Update{Message: &models.Message{}}
	if w.Allows(u) {
		t.Error("expected rejected when sender is zero")
	}
}

func TestAllows_EmptyUpdate(t *testing.T) {
	t.Parallel()
	w := bot.NewWhitelist([]int64{42})
	if w.Allows(&models.Update{}) {
		t.Error("expected rejected for empty update")
	}
}

func TestMembers_ContainsAllIDs(t *testing.T) {
	t.Parallel()
	in := []int64{1, 2, 3}
	w := bot.NewWhitelist(in)
	got := w.Members()
	set := map[int64]bool{}
	for _, id := range got {
		set[id] = true
	}
	for _, want := range in {
		if !set[want] {
			t.Errorf("missing %d in %v", want, got)
		}
	}
}
