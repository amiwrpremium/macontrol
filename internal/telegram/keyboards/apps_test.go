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

// ---------------- AppsKeepChecklist ----------------

func TestAppsKeepChecklist_DefaultsAllToQuit(t *testing.T) {
	t.Parallel()
	items := []keyboards.AppsKeepItem{
		{Name: "Safari", ShortID: "id-s"},
		{Name: "Mail", ShortID: "id-m"},
		{Name: "Finder", ShortID: "id-f"},
	}
	text, kb := keyboards.AppsKeepChecklist(items, "sess-1")
	if !strings.Contains(text, "Will quit *3* of *3*") {
		t.Errorf("default state should mark all 3 as quit: %q", text)
	}
	// 3 app rows + footer (Quit N) + Cancel + Home = 6.
	if len(kb.InlineKeyboard) != 6 {
		t.Fatalf("expected 6 rows; got %d", len(kb.InlineKeyboard))
	}
	for i, it := range items {
		btn := kb.InlineKeyboard[i][0]
		if !strings.HasPrefix(btn.Text, "✗ ") {
			t.Errorf("row %d should start with ✗ marker: %q", i, btn.Text)
		}
		want := "app:keep-toggle:sess-1:" + it.ShortID
		if btn.CallbackData != want {
			t.Errorf("row %d callback = %q want %q", i, btn.CallbackData, want)
		}
	}
}

func TestAppsKeepChecklist_KeptRowsShowCheckmark(t *testing.T) {
	t.Parallel()
	items := []keyboards.AppsKeepItem{
		{Name: "Safari", ShortID: "id-s", Kept: true},
		{Name: "Mail", ShortID: "id-m"},
	}
	text, kb := keyboards.AppsKeepChecklist(items, "sess")
	if !strings.Contains(text, "Will quit *1* of *2*") {
		t.Errorf("one quit, one keep: %q", text)
	}
	if !strings.HasPrefix(kb.InlineKeyboard[0][0].Text, "✓ ") {
		t.Errorf("kept row should show ✓: %q", kb.InlineKeyboard[0][0].Text)
	}
	if !strings.HasPrefix(kb.InlineKeyboard[1][0].Text, "✗ ") {
		t.Errorf("non-kept row should show ✗: %q", kb.InlineKeyboard[1][0].Text)
	}
}

func TestAppsKeepChecklist_FooterReflectsCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		items   []keyboards.AppsKeepItem
		wantBtn string
	}{
		{
			name:    "all-keep_shows_nothing-to-quit",
			items:   []keyboards.AppsKeepItem{{Name: "A", ShortID: "a", Kept: true}},
			wantBtn: "Nothing to quit",
		},
		{
			name:    "one-quit_singular",
			items:   []keyboards.AppsKeepItem{{Name: "A", ShortID: "a"}, {Name: "B", ShortID: "b", Kept: true}},
			wantBtn: "Quit 1 app",
		},
		{
			name:    "many-quit_plural",
			items:   []keyboards.AppsKeepItem{{Name: "A", ShortID: "a"}, {Name: "B", ShortID: "b"}, {Name: "C", ShortID: "c"}},
			wantBtn: "Quit 3 apps",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, kb := keyboards.AppsKeepChecklist(c.items, "sess")
			footerRow := kb.InlineKeyboard[len(c.items)]
			if !strings.Contains(footerRow[0].Text, c.wantBtn) {
				t.Errorf("footer = %q want substring %q", footerRow[0].Text, c.wantBtn)
			}
		})
	}
}

// ---------------- AppsKeepConfirm ----------------

func TestAppsKeepConfirm_HeaderAndBody(t *testing.T) {
	t.Parallel()
	text, kb := keyboards.AppsKeepConfirm([]string{"Safari", "Mail"}, []string{"Finder"}, "sess-9")
	if !strings.Contains(text, "Quit 2 apps") {
		t.Errorf("header missing count: %q", text)
	}
	if !strings.Contains(text, "Will quit") || !strings.Contains(text, "Safari") || !strings.Contains(text, "Mail") {
		t.Errorf("missing to-quit list: %q", text)
	}
	if !strings.Contains(text, "Will keep") || !strings.Contains(text, "Finder") {
		t.Errorf("missing to-keep list: %q", text)
	}
	row := kb.InlineKeyboard[0]
	if row[0].CallbackData != "app:keep-execute:sess-9:ok" {
		t.Errorf("confirm callback wrong: %q", row[0].CallbackData)
	}
	if row[1].CallbackData != "app:keep-back:sess-9" {
		t.Errorf("cancel callback wrong: %q", row[1].CallbackData)
	}
	assertAllRoundtrip(t, kb)
}

func TestAppsKeepConfirm_SingularPlural(t *testing.T) {
	t.Parallel()
	text, _ := keyboards.AppsKeepConfirm([]string{"Safari"}, nil, "s")
	if !strings.Contains(text, "Quit 1 app*?") {
		t.Errorf("singular wrong: %q", text)
	}
	text, _ = keyboards.AppsKeepConfirm(nil, []string{"Finder"}, "s")
	if !strings.Contains(text, "Quit 0 apps*?") {
		t.Errorf("zero wrong: %q", text)
	}
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
