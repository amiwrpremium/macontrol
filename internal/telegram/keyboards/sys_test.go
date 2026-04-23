package keyboards

import (
	"strconv"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/system"
	"github.com/amiwrpremium/macontrol/internal/telegram/callbacks"
)

// ---------------- leafOf (basename helper) ----------------

func TestLeafOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"/Applications/Foo.app/Contents/MacOS/Foo", "Foo"},
		{"/usr/bin/ssh", "ssh"},
		{"WindowServer", "WindowServer"},
		{"", ""},
		{"./relative/path", "path"},
		{"relative-no-slash", "relative-no-slash"},
		{"/", ""},
		{"/just-leaf", "just-leaf"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := leafOf(c.in); got != c.want {
				t.Errorf("leafOf(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

// ---------------- SystemTopList ----------------

func TestSystemTopList_RendersOneRowPerProc(t *testing.T) {
	t.Parallel()
	procs := []system.Process{
		{PID: 100, CPU: 12.3, Mem: 2.1, Command: "/usr/bin/ssh"},
		{PID: 200, CPU: 5.0, Mem: 1.0, Command: "/Applications/Foo.app/Contents/MacOS/Foo"},
	}
	kb := SystemTopList(procs)
	if kb == nil {
		t.Fatal("nil kb")
	}
	// 2 process rows + 1 (refresh+back) + 1 (Home).
	if len(kb.InlineKeyboard) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(kb.InlineKeyboard))
	}
	// First proc row: PID button label + correct callback.
	row0 := kb.InlineKeyboard[0]
	if len(row0) != 1 {
		t.Fatalf("row0 len = %d", len(row0))
	}
	if row0[0].CallbackData != "sys:proc:100" {
		t.Errorf("row0 cb = %q", row0[0].CallbackData)
	}
	for _, want := range []string{"100", "12.3", "ssh"} {
		if !strings.Contains(row0[0].Text, want) {
			t.Errorf("row0 text missing %q: %q", want, row0[0].Text)
		}
	}
	// Second row uses leaf-of for app paths.
	row1 := kb.InlineKeyboard[1]
	if !strings.Contains(row1[0].Text, "Foo") {
		t.Errorf("row1 should contain leaf 'Foo'; got %q", row1[0].Text)
	}
	if row1[0].CallbackData != "sys:proc:200" {
		t.Errorf("row1 cb = %q", row1[0].CallbackData)
	}
	// Refresh + Back row (action carries "top").
	rowRB := kb.InlineKeyboard[2]
	if len(rowRB) != 2 {
		t.Fatalf("refresh+back row should have 2 buttons, got %d", len(rowRB))
	}
	if rowRB[0].CallbackData != "sys:top" {
		t.Errorf("refresh cb = %q (want sys:top)", rowRB[0].CallbackData)
	}
	if rowRB[1].CallbackData != "sys:open" {
		t.Errorf("back cb = %q (want sys:open)", rowRB[1].CallbackData)
	}
	// Trailing nav row contains Home.
	last := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if last[0].CallbackData != "nav:home" {
		t.Errorf("nav cb = %q", last[0].CallbackData)
	}
}

func TestSystemTopList_EmptyProcs(t *testing.T) {
	t.Parallel()
	kb := SystemTopList(nil)
	// Just Refresh+Back + Home row.
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows when procs empty, got %d", len(kb.InlineKeyboard))
	}
	if kb.InlineKeyboard[0][0].CallbackData != "sys:top" {
		t.Errorf("refresh cb = %q", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestSystemTopList_AllCallbacksRoundtrip(t *testing.T) {
	t.Parallel()
	procs := []system.Process{
		{PID: 1, CPU: 0.0, Mem: 0.0, Command: "init"},
		{PID: 99999, CPU: 100.0, Mem: 99.9, Command: "/x/y/z"},
	}
	kb := SystemTopList(procs)
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if b.CallbackData == "" {
				continue
			}
			if _, err := callbacks.Decode(b.CallbackData); err != nil {
				t.Errorf("callback %q does not decode: %v", b.CallbackData, err)
			}
			if len(b.CallbackData) > callbacks.MaxCallbackDataBytes {
				t.Errorf("callback %q exceeds 64B (%d)", b.CallbackData, len(b.CallbackData))
			}
		}
	}
}

// ---------------- SystemProcPanel ----------------

func TestSystemProcPanel_Layout(t *testing.T) {
	t.Parallel()
	pid := 4242
	kb := SystemProcPanel(pid)
	if kb == nil {
		t.Fatal("nil kb")
	}
	// Row0: SIGTERM + Force Kill. Row1: Refresh + Back. Row2: Home.
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}
	wantPID := strconv.Itoa(pid)
	row0 := kb.InlineKeyboard[0]
	if row0[0].CallbackData != "sys:kill-pid:"+wantPID {
		t.Errorf("kill-pid cb = %q", row0[0].CallbackData)
	}
	if row0[1].CallbackData != "sys:kill9:"+wantPID {
		t.Errorf("kill9 cb = %q", row0[1].CallbackData)
	}
	if !strings.Contains(row0[0].Text, "SIGTERM") {
		t.Errorf("SIGTERM label missing: %q", row0[0].Text)
	}
	if !strings.Contains(row0[1].Text, "Force Kill") {
		t.Errorf("Force Kill label missing: %q", row0[1].Text)
	}
	row1 := kb.InlineKeyboard[1]
	if row1[0].CallbackData != "sys:proc:"+wantPID {
		t.Errorf("refresh cb = %q", row1[0].CallbackData)
	}
	if row1[1].CallbackData != "sys:top" {
		t.Errorf("back cb = %q (want sys:top)", row1[1].CallbackData)
	}
	if kb.InlineKeyboard[2][0].CallbackData != "nav:home" {
		t.Errorf("nav cb = %q", kb.InlineKeyboard[2][0].CallbackData)
	}
}

func TestSystemProcPanel_AllCallbacksRoundtrip(t *testing.T) {
	t.Parallel()
	for _, pid := range []int{1, 100, 99999} {
		kb := SystemProcPanel(pid)
		for _, row := range kb.InlineKeyboard {
			for _, b := range row {
				if _, err := callbacks.Decode(b.CallbackData); err != nil {
					t.Errorf("pid=%d callback %q does not decode: %v", pid, b.CallbackData, err)
				}
			}
		}
	}
}

// ---------------- SystemKillConfirm ----------------

func TestSystemKillConfirm_WithName(t *testing.T) {
	t.Parallel()
	text, kb := SystemKillConfirm(123, "Safari")
	for _, want := range []string{"Force kill", "PID 123", "Safari", "SIGKILL"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q: %q", want, text)
		}
	}
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected single row of 2 buttons; got %+v", kb.InlineKeyboard)
	}
	confirm := kb.InlineKeyboard[0][0]
	cancel := kb.InlineKeyboard[0][1]
	if confirm.CallbackData != "sys:kill9:123:ok" {
		t.Errorf("confirm cb = %q", confirm.CallbackData)
	}
	if cancel.CallbackData != "sys:proc:123" {
		t.Errorf("cancel cb = %q (must NOT route to nav:home)", cancel.CallbackData)
	}
	if !strings.Contains(confirm.Text, "Confirm") {
		t.Errorf("confirm label = %q", confirm.Text)
	}
	if !strings.Contains(cancel.Text, "Cancel") {
		t.Errorf("cancel label = %q", cancel.Text)
	}
}

func TestSystemKillConfirm_EmptyName(t *testing.T) {
	t.Parallel()
	text, _ := SystemKillConfirm(7, "")
	if !strings.Contains(text, "(unknown)") {
		t.Errorf("expected '(unknown)' fallback for empty name; got %q", text)
	}
	if !strings.Contains(text, "PID 7") {
		t.Errorf("text missing PID; got %q", text)
	}
}

func TestSystemKillConfirm_AllCallbacksRoundtrip(t *testing.T) {
	t.Parallel()
	_, kb := SystemKillConfirm(42, "x")
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if _, err := callbacks.Decode(b.CallbackData); err != nil {
				t.Errorf("callback %q does not decode: %v", b.CallbackData, err)
			}
		}
	}
}
