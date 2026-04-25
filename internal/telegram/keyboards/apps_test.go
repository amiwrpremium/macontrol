package keyboards_test

import (
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/keyboards"
)

// ---------------- AppsList ----------------

func TestAppsList_RendersOneRowPerApp(t *testing.T) {
	t.Parallel()
	items := []keyboards.AppListItem{
		{Name: "Safari", PID: 1234, ShortID: "id-safari"},
		{Name: "Mail", PID: 2345, Hidden: true, ShortID: "id-mail"},
	}
	text, kb := keyboards.AppsList(items, 0, 1, 2)
	if !strings.Contains(text, "Page 1/1") {
		t.Errorf("missing pager fragment: %q", text)
	}
	if !strings.Contains(text, "2 running") {
		t.Errorf("missing total: %q", text)
	}
	// 2 app rows + Quit-all + Refresh/Back + Home = 5 rows.
	if len(kb.InlineKeyboard) != 5 {
		t.Fatalf("expected 5 rows; got %d", len(kb.InlineKeyboard))
	}
	if got := kb.InlineKeyboard[0][0].Text; !strings.Contains(got, "Safari") {
		t.Errorf("row 0 missing Safari: %q", got)
	}
	if got := kb.InlineKeyboard[1][0].Text; !strings.Contains(got, "·") || !strings.Contains(got, "Mail") {
		t.Errorf("hidden Mail row should have hidden marker: %q", got)
	}
	if got := kb.InlineKeyboard[0][0].CallbackData; got != "app:show:id-safari" {
		t.Errorf("Safari callback wrong: %q", got)
	}
	assertAllRoundtrip(t, kb)
}

func TestAppsList_PagerOnlyAppearsWhenMultiPage(t *testing.T) {
	t.Parallel()
	items := []keyboards.AppListItem{{Name: "Safari", PID: 1, ShortID: "x"}}
	_, kb := keyboards.AppsList(items, 0, 1, 1)
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.Contains(b.Text, "Prev") || strings.Contains(b.Text, "Next") {
				t.Fatalf("single-page list should not show pager: %q", b.Text)
			}
		}
	}
}

func TestAppsList_PagerEdgesOmitOppositeArrow(t *testing.T) {
	t.Parallel()
	items := []keyboards.AppListItem{{Name: "A", PID: 1, ShortID: "a"}}
	_, firstKB := keyboards.AppsList(items, 0, 3, 30)
	_, lastKB := keyboards.AppsList(items, 2, 3, 30)
	hasText := func(kb *struct{}, _ string) bool { return false }
	_ = hasText
	firstHas := func(s string) bool {
		for _, row := range firstKB.InlineKeyboard {
			for _, b := range row {
				if strings.Contains(b.Text, s) {
					return true
				}
			}
		}
		return false
	}
	lastHas := func(s string) bool {
		for _, row := range lastKB.InlineKeyboard {
			for _, b := range row {
				if strings.Contains(b.Text, s) {
					return true
				}
			}
		}
		return false
	}
	if firstHas("Prev") {
		t.Error("page 0 should not show Prev")
	}
	if !firstHas("Next") {
		t.Error("page 0 should show Next")
	}
	if !lastHas("Prev") {
		t.Error("last page should show Prev")
	}
	if lastHas("Next") {
		t.Error("last page should not show Next")
	}
}

func TestAppsList_EmptyShowsHint(t *testing.T) {
	t.Parallel()
	text, kb := keyboards.AppsList(nil, 0, 0, 0)
	if !strings.Contains(text, "No running apps") {
		t.Errorf("empty list should show hint: %q", text)
	}
	if !strings.Contains(text, "Page 1/1") {
		t.Errorf("empty list should still show Page 1/1: %q", text)
	}
	assertAllRoundtrip(t, kb)
}

func TestAppsList_QuitAllExceptRow(t *testing.T) {
	t.Parallel()
	_, kb := keyboards.AppsList(nil, 0, 1, 0)
	found := false
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.Contains(b.Text, "Quit all except") {
				found = true
				if b.CallbackData != "app:keep" {
					t.Errorf("quit-all-except callback wrong: %q", b.CallbackData)
				}
			}
		}
	}
	if !found {
		t.Error("quit-all-except button missing")
	}
}

// ---------------- AppPanel ----------------

func TestAppPanel_HeaderAndButtons(t *testing.T) {
	t.Parallel()
	text, kb := keyboards.AppPanel("Safari", 1234, "id-safari")
	if !strings.Contains(text, "Safari") || !strings.Contains(text, "1234") {
		t.Errorf("header missing fields: %q", text)
	}
	// Quit + Force Quit + Hide + Refresh+Back + Home = 4 rows.
	if len(kb.InlineKeyboard) != 4 {
		t.Fatalf("expected 4 rows; got %d", len(kb.InlineKeyboard))
	}
	row0 := kb.InlineKeyboard[0]
	if len(row0) != 2 {
		t.Fatalf("row 0 should have 2 buttons; got %d", len(row0))
	}
	if row0[0].CallbackData != "app:quit:id-safari" {
		t.Errorf("Quit callback wrong: %q", row0[0].CallbackData)
	}
	if row0[1].CallbackData != "app:force:id-safari" {
		t.Errorf("Force Quit callback wrong: %q", row0[1].CallbackData)
	}
	row1 := kb.InlineKeyboard[1]
	if row1[0].CallbackData != "app:hide:id-safari" {
		t.Errorf("Hide callback wrong: %q", row1[0].CallbackData)
	}
	row2 := kb.InlineKeyboard[2]
	if row2[1].CallbackData != "app:open" {
		t.Errorf("Back callback wrong: %q", row2[1].CallbackData)
	}
	assertAllRoundtrip(t, kb)
}

// ---------------- AppQuitConfirm ----------------

func TestAppQuitConfirm_Shape(t *testing.T) {
	t.Parallel()
	text, kb := keyboards.AppQuitConfirm("Safari", "id-safari")
	if !strings.Contains(text, "Quit Safari") {
		t.Errorf("header missing app name: %q", text)
	}
	if !strings.Contains(text, "unsaved-document") {
		t.Errorf("graceful warning missing: %q", text)
	}
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1×2 keyboard; got %dx?", len(kb.InlineKeyboard))
	}
	confirm := kb.InlineKeyboard[0][0].CallbackData
	if confirm != "app:quit:id-safari:ok" {
		t.Errorf("confirm callback wrong: %q", confirm)
	}
	cancel := kb.InlineKeyboard[0][1].CallbackData
	if cancel != "app:show:id-safari" {
		t.Errorf("cancel should return to per-app panel; got %q", cancel)
	}
	assertAllRoundtrip(t, kb)
}

// ---------------- AppForceConfirm ----------------

func TestAppForceConfirm_Shape(t *testing.T) {
	t.Parallel()
	text, kb := keyboards.AppForceConfirm("Safari", 1234, "id-safari")
	if !strings.Contains(text, "Force Quit Safari") {
		t.Errorf("header missing app name: %q", text)
	}
	if !strings.Contains(text, "1234") {
		t.Errorf("header missing PID: %q", text)
	}
	if !strings.Contains(text, "SIGKILL") {
		t.Errorf("header missing SIGKILL warning: %q", text)
	}
	confirm := kb.InlineKeyboard[0][0].CallbackData
	if confirm != "app:force:id-safari:ok" {
		t.Errorf("confirm callback wrong: %q", confirm)
	}
	cancel := kb.InlineKeyboard[0][1].CallbackData
	if cancel != "app:show:id-safari" {
		t.Errorf("cancel should return to per-app panel; got %q", cancel)
	}
	assertAllRoundtrip(t, kb)
}

// ---------------- callback budget ----------------

func TestApps_CallbackDataWithinBudget(t *testing.T) {
	t.Parallel()
	// Worst case: 10-char ShortID. Largest action is "force"
	// (5) → app(3) + : + force(5) + : + 10-char id + : + ok(2)
	// = 23 bytes — well under 64. Sanity-check via a real
	// keyboard build.
	items := []keyboards.AppListItem{{Name: "Visual Studio Code", PID: 99999, Hidden: true, ShortID: "abcdefghij"}}
	_, kb := keyboards.AppsList(items, 9, 10, 150)
	assertAllRoundtrip(t, kb)
	_, panel := keyboards.AppPanel("Visual Studio Code", 99999, "abcdefghij")
	assertAllRoundtrip(t, panel)
	_, quit := keyboards.AppQuitConfirm("Visual Studio Code", "abcdefghij")
	assertAllRoundtrip(t, quit)
	_, force := keyboards.AppForceConfirm("Visual Studio Code", 99999, "abcdefghij")
	assertAllRoundtrip(t, force)
}
