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
