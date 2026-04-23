package tools_test

import (
	"context"
	"errors"
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

func TestDisksList_OnlyUserFacingMounts(t *testing.T) {
	t.Parallel()
	// Real macOS 26 shape (matches the issue #56 capture): root,
	// devfs, several /System/Volumes/* and /Library/Developer/...
	// CoreSimulator mounts, plus one user-facing /Volumes/External.
	// Only / and /Volumes/External should survive the new filter.
	out := "Filesystem        Size    Used   Avail Capacity iused ifree %iused  Mounted on\n" +
		"/dev/disk3s1s1   460Gi    17Gi    13Gi    57%    447k  135M    0%   /\n" +
		"devfs            221Ki   221Ki     0Bi   100%     766     0  100%   /dev\n" +
		"/dev/disk3s6     460Gi   3.0Gi    13Gi    19%       3  135M    0%   /System/Volumes/VM\n" +
		"/dev/disk3s2     460Gi    16Gi    13Gi    55%    2.0k  135M    0%   /System/Volumes/Preboot\n" +
		"/dev/disk3s5     460Gi   409Gi    13Gi    97%    3.2M  135M    2%   /System/Volumes/Data\n" +
		"/dev/disk3s1     460Gi    17Gi    13Gi    57%    458k  135M    0%   /System/Volumes/Update/mnt1\n" +
		"/dev/disk5s1     8.5Gi   8.2Gi   251Mi    98%      13  2.6M    0%   /Library/Developer/CoreSimulator/Cryptex/Images/bundle/SimRuntimeBundle\n" +
		"/dev/disk7s1      19Gi    19Gi   495Mi    98%    559k  5.1M   10%   /Library/Developer/CoreSimulator/Volumes/iOS_22F77\n" +
		"/dev/disk2s1     500Gi   300Gi   200Gi    60%    5.0k   10k   33%   /Volumes/External\n"
	f := runner.NewFake().On("df -h", out, nil)
	vols, err := tools.New(f).DisksList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mounts := make([]string, 0, len(vols))
	for _, v := range vols {
		mounts = append(mounts, v.MountedOn)
	}
	want := []string{"/", "/Volumes/External"}
	if len(mounts) != len(want) {
		t.Fatalf("got mounts %v; want exactly %v", mounts, want)
	}
	for i, m := range want {
		if mounts[i] != m {
			t.Errorf("mount[%d] = %q; want %q", i, mounts[i], m)
		}
	}
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

func TestDiskInfo_ParsesRealOutput(t *testing.T) {
	t.Parallel()
	// Real macOS 26 diskutil info / output (matches the issue #56
	// capture). Parser should pull volume name, FS, sizes, and the
	// removable / internal / SSD flags out cleanly.
	out := "" +
		"   Device Identifier:         disk3s1s1\n" +
		"   Device Node:               /dev/disk3s1s1\n" +
		"   Volume Name:               Macintosh HD\n" +
		"   Mounted:                   Yes\n" +
		"   Mount Point:               /\n" +
		"   File System Personality:   APFS\n" +
		"   Disk Size:                 494.4 GB (494384795648 Bytes) (exactly 965595304 512-Byte-Units)\n" +
		"   Volume Used Space:         17.9 GB (17851154432 Bytes) (exactly 34865536 512-Byte-Units)\n" +
		"   Container Free Space:      13.8 GB (13812297728 Bytes) (exactly 26977144 512-Byte-Units)\n" +
		"   Volume Read-Only:          Yes (read-only mount flag set)\n" +
		"   Device Location:           Internal\n" +
		"   Removable Media:           Fixed\n" +
		"   Solid State:               Yes\n"
	f := runner.NewFake().On("diskutil info /", out, nil)
	d, err := tools.New(f).DiskInfo(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if d.VolumeName != "Macintosh HD" {
		t.Errorf("name = %q", d.VolumeName)
	}
	if d.FSType != "APFS" {
		t.Errorf("fs = %q", d.FSType)
	}
	if d.Device != "/dev/disk3s1s1" {
		t.Errorf("device = %q", d.Device)
	}
	if d.DiskSize != "494.4 GB" {
		t.Errorf("disk size = %q (parens-trim should strip the (… Bytes) suffix)", d.DiskSize)
	}
	if d.UsedSpace != "17.9 GB" || d.FreeSpace != "13.8 GB" {
		t.Errorf("used=%q free=%q", d.UsedSpace, d.FreeSpace)
	}
	if d.Removable {
		t.Error("Removable Media: Fixed → Removable should be false")
	}
	if !d.Internal {
		t.Error("Device Location: Internal → Internal should be true")
	}
	if !d.ReadOnly {
		t.Error("Volume Read-Only: Yes → ReadOnly should be true")
	}
	if !d.SolidState {
		t.Error("Solid State: Yes → SolidState should be true")
	}
}

func TestDiskInfo_RemovableExternal(t *testing.T) {
	t.Parallel()
	// Mock an external removable USB stick.
	out := "   Volume Name:               BACKUP\n" +
		"   Mount Point:               /Volumes/BACKUP\n" +
		"   Removable Media:           Removable\n" +
		"   Device Location:           External\n" +
		"   Solid State:               No\n"
	f := runner.NewFake().On("diskutil info /Volumes/BACKUP", out, nil)
	d, _ := tools.New(f).DiskInfo(context.Background(), "/Volumes/BACKUP")
	if !d.Removable {
		t.Error("expected Removable=true for external USB")
	}
	if d.Internal {
		t.Error("expected Internal=false for external USB")
	}
	if d.SolidState {
		t.Error("expected SolidState=false for spinning USB")
	}
}

func TestDiskInfo_RejectsEmptyMount(t *testing.T) {
	t.Parallel()
	if _, err := tools.New(runner.NewFake()).DiskInfo(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty mount")
	}
}

func TestEjectDisk(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("diskutil eject /Volumes/USB", "", nil)
	if err := tools.New(f).EjectDisk(context.Background(), "/Volumes/USB"); err != nil {
		t.Fatal(err)
	}
}

func TestEjectDisk_Propagates(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("diskutil eject /Volumes/USB", "", errors.New("Disk in use"))
	if err := tools.New(f).EjectDisk(context.Background(), "/Volumes/USB"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEjectDisk_RejectsEmptyMount(t *testing.T) {
	t.Parallel()
	if err := tools.New(runner.NewFake()).EjectDisk(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty mount")
	}
}

func TestOpenInFinder(t *testing.T) {
	t.Parallel()
	f := runner.NewFake().On("open /Volumes/USB", "", nil)
	if err := tools.New(f).OpenInFinder(context.Background(), "/Volumes/USB"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenInFinder_RejectsEmptyMount(t *testing.T) {
	t.Parallel()
	if err := tools.New(runner.NewFake()).OpenInFinder(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty mount")
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
