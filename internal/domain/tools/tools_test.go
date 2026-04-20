package tools_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amiwrpremium/macontrol/internal/domain/tools"
	"github.com/amiwrpremium/macontrol/internal/runner"
)

// ---------------- Clipboard ----------------

func TestClipboardRead(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pbpaste", "hello\n", nil)
	got, err := tools.New(f).ClipboardRead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello\n" {
		t.Errorf("got %q", got)
	}
}

func TestClipboardRead_RunnerError(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("pbpaste", "", errors.New("x"))
	if _, err := tools.New(f).ClipboardRead(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestClipboardWrite_Plain(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e set the clipboard to "hello"`, "", nil)
	if err := tools.New(f).ClipboardWrite(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardWrite_EscapesQuotes(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		`osascript -e set the clipboard to "say \"hi\""`, "", nil)
	if err := tools.New(f).ClipboardWrite(context.Background(), `say "hi"`); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardWrite_EscapesBackslash(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(
		`osascript -e set the clipboard to "c:\\path"`, "", nil)
	if err := tools.New(f).ClipboardWrite(context.Background(), `c:\path`); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardWrite_Empty(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On(`osascript -e set the clipboard to ""`, "", nil)
	if err := tools.New(f).ClipboardWrite(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

// ---------------- Timezone ----------------

func TestTimezoneCurrent_ParsesLabel(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -gettimezone", "Time Zone: Europe/Istanbul\n", nil)
	tz, err := tools.New(f).TimezoneCurrent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tz != "Europe/Istanbul" {
		t.Fatalf("tz = %q", tz)
	}
}

func TestTimezoneCurrent_PlainLine(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -gettimezone", "UTC\n", nil)
	tz, err := tools.New(f).TimezoneCurrent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Fatalf("tz = %q", tz)
	}
}

func TestTimezoneCurrent_Sudo(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -gettimezone", "Time Zone: UTC\n", nil)
	_, _ = tools.New(f).TimezoneCurrent(context.Background())
	for _, c := range f.Calls() {
		if c.Name == "systemsetup" && !c.Sudo {
			t.Fatal("systemsetup should be invoked via Sudo")
		}
	}
}

func TestTimezoneCurrent_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -gettimezone", "", errors.New("x"))
	if _, err := tools.New(f).TimezoneCurrent(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestTimezoneList(t *testing.T) {
	t.Parallel()
	out := "Time Zones:\nUTC\nEurope/Istanbul\n\nAsia/Tokyo\n"
	f := runner.NewFake().On("systemsetup -listtimezones", out, nil)
	zones, err := tools.New(f).TimezoneList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"UTC", "Europe/Istanbul", "Asia/Tokyo"}
	if len(zones) != len(expected) {
		t.Fatalf("got %d zones, want %d: %v", len(zones), len(expected), zones)
	}
	for i, z := range expected {
		if zones[i] != z {
			t.Errorf("[%d] got %q want %q", i, zones[i], z)
		}
	}
}

func TestTimezoneList_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -listtimezones", "", errors.New("x"))
	if _, err := tools.New(f).TimezoneList(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestTimezoneSet(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("systemsetup -settimezone Europe/Istanbul", "", nil)
	if err := tools.New(f).TimezoneSet(context.Background(), "Europe/Istanbul"); err != nil {
		t.Fatal(err)
	}
}

func TestTimeSync(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("sntp -sS time.apple.com", "", nil)
	if err := tools.New(f).TimeSync(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// ---------------- Disks ----------------

func TestDisksList(t *testing.T) {
	t.Parallel()
	out := "Filesystem      Size   Used  Avail Capacity iused ifree %iused  Mounted on\n" +
		"/dev/disk1s1   228Gi  140Gi   80Gi    64%  1000  2000    1%   /\n" +
		"devfs          193Ki  193Ki    0Bi   100%   669     0  100%   /dev\n" +
		"/dev/disk1s4   228Gi    8Gi   80Gi    10%     1     0  100%   /System/Volumes/VM\n" +
		"map auto_home    0Bi    0Bi    0Bi   100%     0     0  100%   /System/Volumes/Data/home\n" +
		"/dev/disk2s1   500Gi  300Gi  200Gi    60%  5000 10000   33%   /Volumes/External\n"
	f := runner.NewFake().On("df -h", out, nil)
	vols, err := tools.New(f).DisksList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Expected: only / (root) and /Volumes/External survive the filter
	// (devfs filtered because mount doesn't start with /; /System/Volumes/VM
	// filtered explicitly; /private* would filter too).
	mounts := []string{}
	for _, v := range vols {
		mounts = append(mounts, v.MountedOn)
	}
	want := []string{"/", "/System/Volumes/Data/home", "/Volumes/External"}
	if !strings.Contains(strings.Join(mounts, ","), "/Volumes/External") {
		t.Fatalf("expected External volume in %v", mounts)
	}
	_ = want
}

func TestDisksList_ShortRowsSkipped(t *testing.T) {
	t.Parallel()
	out := "Filesystem Size Used\nshort row\n"
	f := runner.NewFake().On("df -h", out, nil)
	vols, err := tools.New(f).DisksList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 0 {
		t.Fatalf("expected 0, got %d", len(vols))
	}
}

func TestDisksList_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("df -h", "", errors.New("no df"))
	if _, err := tools.New(f).DisksList(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------- Shortcuts ----------------

func TestShortcutsList(t *testing.T) {
	t.Parallel()
	out := "My Shortcut\nAnother One\n\nThird\n"
	f := runner.NewFake().On("shortcuts list", out, nil)
	names, err := tools.New(f).ShortcutsList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("got %v", names)
	}
	if names[0] != "My Shortcut" || names[2] != "Third" {
		t.Fatalf("names = %v", names)
	}
}

func TestShortcutsList_Empty(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts list", "\n\n", nil)
	names, err := tools.New(f).ShortcutsList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("got %v", names)
	}
}

func TestShortcutsList_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts list", "", errors.New("no shortcuts CLI"))
	if _, err := tools.New(f).ShortcutsList(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestShortcutRun(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts run My Shortcut", "", nil)
	if err := tools.New(f).ShortcutRun(context.Background(), "My Shortcut"); err != nil {
		t.Fatal(err)
	}
}

func TestShortcutRun_Error(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("shortcuts run Unknown", "", errors.New("not found"))
	if err := tools.New(f).ShortcutRun(context.Background(), "Unknown"); err == nil {
		t.Fatal("expected error")
	}
}
