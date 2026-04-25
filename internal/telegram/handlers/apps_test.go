package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/telegram/handlers"
)

// appsRunningCmd is the runner.Fake key for the apps listing
// osascript invocation. Mirrors the unexported runningScript
// constant in the apps package; tests hard-code the literal so a
// refactor that changes the script fails the test loudly rather
// than silently.
const appsRunningCmd = "osascript -e tell application \"System Events\"\n" +
	"set out to \"\"\n" +
	"repeat with p in (processes whose background only is false)\n" +
	"set out to out & (name of p) & \"|\" & (unix id of p) & \"|\" & ((not (visible of p)) as text) & linefeed\n" +
	"end repeat\n" +
	"return out\n" +
	"end tell"

// appsListingFixture is the canned osascript output used by
// every test that just needs a stable list.
const appsListingFixture = "Safari|1234|false\nMail|2345|true\nFinder|3456|false\n"

// stageRunning stages the listing fixture against h.Fake so the
// handler can call Running() and get a populated list back. It
// does NOT clear other rules.
func stageRunning(h *harness, out string) {
	h.Fake.On(appsRunningCmd, out, nil)
}

// ---------------- open + list ----------------

func TestApps_Open_RendersListWithRowPerApp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:open")); err != nil {
		t.Fatal(err)
	}
	edits := h.Recorder.ByMethod("editMessageText")
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit; got %d", len(edits))
	}
	text := edits[0].Fields["text"]
	if !strings.Contains(text, "3 running") {
		t.Errorf("header should report 3 running; got %q", text)
	}
	mk := edits[0].Fields["reply_markup"]
	for _, want := range []string{"Safari", "Mail", "Finder", "Quit all except"} {
		if !strings.Contains(mk, want) {
			t.Errorf("keyboard missing %q: %s", want, mk)
		}
	}
}

func TestApps_Open_EmptyListShowsHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, "")

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:open")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "No running apps") {
		t.Errorf("empty hint missing: %q", text)
	}
}

func TestApps_Open_RunnerErrorSurfacesTCCHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Fake.On(appsRunningCmd, "", errors.New("osascript: tcc denied"))

	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:open"))
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "TCC") {
		t.Errorf("error path should mention TCC: %q", text)
	}
}

func TestApps_List_PaginatesByPageArg(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	// 17 apps → 2 pages at page-size 15.
	var sb strings.Builder
	for i := 0; i < 17; i++ {
		fmt.Fprintf(&sb, "App%02d|%d|false\n", i, 1000+i)
	}
	stageRunning(h, sb.String())

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:list:1")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Page 2/2") {
		t.Errorf("expected page 2/2 header; got %q", text)
	}
}

// ---------------- show ----------------

func TestApps_Show_RendersPanelForName(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	id := h.Deps.ShortMap.Put("Safari")

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:show:"+id)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Safari") || !strings.Contains(text, "1234") {
		t.Errorf("panel header missing name/PID: %q", text)
	}
}

func TestApps_Show_NotRunningShowsHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	id := h.Deps.ShortMap.Put("Ghost")

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:show:"+id)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "not running anymore") {
		t.Errorf("missing not-running hint: %q", text)
	}
}

func TestApps_Show_ExpiredShortMapEntry(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	// No ShortMap.Put; the id resolves to ("", false).
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:show:nope")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "session expired") {
		t.Errorf("missing expiry hint: %q", text)
	}
}

// ---------------- quit (graceful) confirm flow ----------------

func TestApps_Quit_FirstTapShowsConfirm(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := h.Deps.ShortMap.Put("Safari")

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:quit:"+id)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Quit Safari") {
		t.Errorf("first tap should show confirm: %q", text)
	}
	// No quit yet: the Quit script must NOT have been run.
	for _, c := range h.Fake.Calls() {
		if strings.Contains(strings.Join(c.Args, " "), `tell application "Safari" to quit`) {
			t.Fatalf("unexpected quit invocation on first tap: %+v", c)
		}
	}
}

func TestApps_Quit_SecondTapExecutes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := h.Deps.ShortMap.Put("Safari")
	h.Fake.
		On(`osascript -e tell application "Safari" to quit`, "", nil).
		On(appsRunningCmd, "Mail|1|false\n", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:quit:"+id+":ok")); err != nil {
		t.Fatal(err)
	}
	// The second tap re-renders the list with the toast banner.
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Quit sent") || !strings.Contains(text, "Safari") {
		t.Errorf("expected toast banner: %q", text)
	}
	// The osascript quit must have been called.
	saw := false
	for _, c := range h.Fake.Calls() {
		if strings.Contains(strings.Join(c.Args, " "), `tell application "Safari" to quit`) {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected quit command to have run")
	}
}

// ---------------- force (SIGKILL) confirm flow ----------------

func TestApps_Force_FirstTapShowsConfirm(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	id := h.Deps.ShortMap.Put("Safari")

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:force:"+id)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Force Quit Safari") || !strings.Contains(text, "1234") {
		t.Errorf("force confirm header wrong: %q", text)
	}
	// No kill should have been called.
	for _, c := range h.Fake.Calls() {
		if c.Name == "kill" {
			t.Fatalf("unexpected kill on first force tap: %+v", c)
		}
	}
}

func TestApps_Force_SecondTapExecutes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	id := h.Deps.ShortMap.Put("Safari")
	h.Fake.On("kill -KILL 1234", "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:force:"+id+":ok")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "SIGKILL sent") || !strings.Contains(text, "1234") {
		t.Errorf("expected SIGKILL toast: %q", text)
	}
	saw := false
	for _, c := range h.Fake.Calls() {
		if c.Name == "kill" && len(c.Args) == 2 && c.Args[0] == "-KILL" && c.Args[1] == "1234" {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected kill -KILL 1234 to have run")
	}
}

// ---------------- hide ----------------

func TestApps_Hide_ExecutesImmediatelyNoConfirm(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	id := h.Deps.ShortMap.Put("Safari")
	h.Fake.On(`osascript -e tell application "System Events" to set visible of process "Safari" to false`, "", nil)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:hide:"+id)); err != nil {
		t.Fatal(err)
	}
	// Toast is "Hidden." — answerCallbackQuery with text.
	saw := false
	for _, c := range h.Recorder.ByMethod("answerCallbackQuery") {
		if c.Fields["text"] == "Hidden." {
			saw = true
			break
		}
	}
	if !saw {
		t.Error("expected Hidden toast on the answerCallbackQuery")
	}
	// Re-renders the per-app panel.
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Safari") {
		t.Errorf("expected re-rendered panel: %q", text)
	}
}

// ---------------- keep (quit all except…) ----------------

func TestApps_Keep_RendersChecklistWithAllAsQuit(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep")); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Will quit <b>3</b> of <b>3</b>") {
		t.Errorf("default kept-set should mark all 3 as quit: %q", text)
	}
}

func TestApps_KeepToggle_AddsAndRemovesNames(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)

	// Open the checklist; save the initial sessionID by digging
	// into the rendered keyboard.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep"))
	mk := h.Recorder.ByMethod("editMessageText")[0].Fields["reply_markup"]
	sess := extractSessionID(t, mk)
	safariID := extractToggleAppID(t, mk, "Safari")

	// Tap to add Safari to KEEP.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id2", "app:keep-toggle:"+sess+":"+safariID))
	text := h.Recorder.ByMethod("editMessageText")[1].Fields["text"]
	if !strings.Contains(text, "Will quit <b>2</b> of <b>3</b>") {
		t.Errorf("after toggle KEEP-Safari, header should say 2 of 3: %q", text)
	}

	// The new keyboard has a new sessionID and refreshed app IDs.
	mk2 := h.Recorder.ByMethod("editMessageText")[1].Fields["reply_markup"]
	sess2 := extractSessionID(t, mk2)
	if sess2 == sess {
		t.Error("sessionID should re-stamp on every toggle")
	}
	safariID2 := extractToggleAppID(t, mk2, "Safari")
	// Tap Safari again to remove from KEEP.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id3", "app:keep-toggle:"+sess2+":"+safariID2))
	text3 := h.Recorder.ByMethod("editMessageText")[2].Fields["text"]
	if !strings.Contains(text3, "Will quit <b>3</b> of <b>3</b>") {
		t.Errorf("after toggle BACK, header should say 3 of 3 again: %q", text3)
	}
}

func TestApps_KeepConfirm_ListsToQuitAndToKeep(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)

	// Open + toggle Mail to keep.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep"))
	mk := h.Recorder.ByMethod("editMessageText")[0].Fields["reply_markup"]
	sess := extractSessionID(t, mk)
	mailID := extractToggleAppID(t, mk, "Mail")
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id2", "app:keep-toggle:"+sess+":"+mailID))
	mk2 := h.Recorder.ByMethod("editMessageText")[1].Fields["reply_markup"]
	sess2 := extractSessionID(t, mk2)

	// Render confirm.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id3", "app:keep-confirm:"+sess2))
	text := h.Recorder.ByMethod("editMessageText")[2].Fields["text"]
	if !strings.Contains(text, "Will quit") || !strings.Contains(text, "Safari") || !strings.Contains(text, "Finder") {
		t.Errorf("missing to-quit names: %q", text)
	}
	if !strings.Contains(text, "Will keep") || !strings.Contains(text, "Mail") {
		t.Errorf("missing to-keep names: %q", text)
	}
}

func TestApps_KeepExecute_QuitsToQuitListAndShowsBanner(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	h.Fake.
		On(`osascript -e tell application "Safari" to quit`, "", nil).
		On(`osascript -e tell application "Finder" to quit`, "", nil)

	// Open + toggle Mail to keep + confirm + execute.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep"))
	mk := h.Recorder.ByMethod("editMessageText")[0].Fields["reply_markup"]
	sess := extractSessionID(t, mk)
	mailID := extractToggleAppID(t, mk, "Mail")
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id2", "app:keep-toggle:"+sess+":"+mailID))
	sess2 := extractSessionID(t, h.Recorder.ByMethod("editMessageText")[1].Fields["reply_markup"])
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id3", "app:keep-execute:"+sess2+":ok")); err != nil {
		t.Fatal(err)
	}

	text := h.Recorder.ByMethod("editMessageText")[2].Fields["text"]
	if !strings.Contains(text, "Sent quit to <b>2</b> apps") {
		t.Errorf("expected '2 apps' banner: %q", text)
	}
	// Both Safari and Finder must have been told to quit; Mail must NOT have.
	saw := map[string]bool{}
	for _, c := range h.Fake.Calls() {
		joined := strings.Join(c.Args, " ")
		if strings.Contains(joined, `tell application "Safari" to quit`) {
			saw["Safari"] = true
		}
		if strings.Contains(joined, `tell application "Finder" to quit`) {
			saw["Finder"] = true
		}
		if strings.Contains(joined, `tell application "Mail" to quit`) {
			t.Fatal("Mail should have been kept, not quit")
		}
	}
	if !saw["Safari"] || !saw["Finder"] {
		t.Errorf("expected both Safari and Finder quit calls; saw %v", saw)
	}
}

func TestApps_KeepExecute_NeedsConfirmation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	sess := h.Deps.ShortMap.Put(`[]`)

	// No "ok" arg → handler refuses.
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep-execute:"+sess))
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "missing confirmation") {
		t.Errorf("expected missing-confirmation error: %q", text)
	}
}

func TestApps_KeepBack_ReturnsToChecklistWithSameSet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	stageRunning(h, appsListingFixture)
	sess := h.Deps.ShortMap.Put(`["Safari"]`)

	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep-back:"+sess)); err != nil {
		t.Fatal(err)
	}
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "Will quit <b>2</b> of <b>3</b>") {
		t.Errorf("kept-Safari state should preserve through Back: %q", text)
	}
}

func TestApps_KeepToggle_ExpiredSessionShowsHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_ = handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:keep-toggle:nope:nope-app"))
	text := h.Recorder.ByMethod("editMessageText")[0].Fields["text"]
	if !strings.Contains(text, "session expired") {
		t.Errorf("expected expiry hint: %q", text)
	}
}

// extractSessionID pulls the sessionID out of the first
// keep-toggle callback in the rendered reply_markup. Mirrors
// how a real Telegram client would parse the keyboard before
// dispatching the next tap.
func extractSessionID(t *testing.T, mk string) string {
	t.Helper()
	prefix := `app:keep-toggle:`
	i := strings.Index(mk, prefix)
	if i < 0 {
		t.Fatalf("no keep-toggle callback in markup: %s", mk)
	}
	start := i + len(prefix)
	end := start
	for end < len(mk) && mk[end] != ':' && mk[end] != '"' {
		end++
	}
	return mk[start:end]
}

// extractToggleAppID returns the per-app ShortID for name from
// the rendered checklist. Walks each keep-toggle callback in
// the JSON-encoded markup and matches the ShortID whose
// preceding text field contains name. Mirrors how a real
// Telegram client would resolve a tap to its callback.
func extractToggleAppID(t *testing.T, mk, name string) string {
	t.Helper()
	prefix := `app:keep-toggle:`
	for i := 0; ; {
		j := strings.Index(mk[i:], prefix)
		if j < 0 {
			break
		}
		start := i + j + len(prefix)
		colon := strings.IndexByte(mk[start:], ':')
		if colon < 0 {
			break
		}
		idStart := start + colon + 1
		idEnd := idStart
		for idEnd < len(mk) && mk[idEnd] != '"' {
			idEnd++
		}
		id := mk[idStart:idEnd]
		// Walk back ~200 chars to catch the preceding "text" field.
		windowStart := 0
		if idStart-200 > 0 {
			windowStart = idStart - 200
		}
		if strings.Contains(mk[windowStart:idStart], name) {
			return id
		}
		i = idEnd
	}
	t.Fatalf("no keep-toggle entry for %q in markup", name)
	return ""
}

// ---------------- unknown action ----------------

func TestApps_UnknownActionToasts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if err := handlers.NewCallbackRouter().Handle(context.Background(), h.Deps,
		newCallbackUpdate("id", "app:bogus")); err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, c := range h.Recorder.ByMethod("answerCallbackQuery") {
		if strings.Contains(c.Fields["text"], "Unknown app action") {
			saw = true
			break
		}
	}
	if !saw {
		t.Error("expected Unknown app action toast")
	}
}
