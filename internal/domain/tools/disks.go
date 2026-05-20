package tools

import (
	"context"
	"fmt"
	"strings"
)

// DiskVolume is one mounted filesystem from the user-facing
// subset of `df -h`. Returned by [Service.DisksList]; the
// keyboard renders one row per volume in the Disks panel.
//
// Lifecycle:
//   - Constructed by Service.DisksList each render. Never cached.
//
// Field roles:
//   - Filesystem is the device path from df's first column,
//     e.g. "/dev/disk3s1s1".
//   - Size / Used / Available / Capacity are the human-readable
//     columns from df (verbatim, including units like "494Gi").
//   - MountedOn is the mount point. The keyboard uses this as
//     the lookup key when the user taps a volume to drill into
//     [DiskDetails].
type DiskVolume struct {
	// Filesystem is the device path, e.g. "/dev/disk3s1s1".
	Filesystem string

	// Size is the human-readable total size column from df.
	Size string

	// Used is the human-readable used-bytes column from df.
	Used string

	// Available is the human-readable free-bytes column from df.
	Available string

	// Capacity is the percent-used column, e.g. "42%".
	Capacity string

	// MountedOn is the mount point, e.g. "/" or "/Volumes/Backup".
	MountedOn string
}

// DiskDetails is the per-disk drill-down view, parsed from
// `diskutil info <mount>`. Returned by [Service.DiskInfo].
//
// Lifecycle:
//   - Constructed by Service.DiskInfo each time the user taps
//     a volume in the Disks panel. Never cached.
//
// Field roles:
//   - VolumeName / MountPoint / Device / FSType are identification
//     fields parsed from the corresponding diskutil keys.
//   - DiskSize / UsedSpace / FreeSpace are size strings stripped
//     of diskutil's verbose " (N Bytes) (exactly …)" suffix via
//     [trimAfterParen].
//   - Removable / Internal / ReadOnly / SolidState are bool
//     classifiers used by the keyboard to decide which action
//     buttons to show (e.g. Eject only on removable volumes).
//   - Raw is the full unparsed `diskutil info` output, kept for
//     fallback display when a parse miss leaves key fields empty.
//
// Empty fields mean diskutil didn't expose that line on this Mac
// or for this volume — partial parses are normal.
type DiskDetails struct {
	// VolumeName is the user-visible volume name, e.g.
	// "Macintosh HD".
	VolumeName string

	// MountPoint is the mount path, e.g. "/" or "/Volumes/Backup".
	MountPoint string

	// Device is the device node, e.g. "/dev/disk3s1s1".
	Device string

	// FSType is the filesystem personality reported by
	// diskutil, e.g. "APFS" / "exFAT" / "Tagged HFS+".
	FSType string

	// DiskSize is the total capacity string with diskutil's
	// trailing byte-count suffix stripped, e.g. "494.4 GB".
	DiskSize string

	// UsedSpace is the used portion of the APFS container with
	// the trailing byte-count stripped.
	UsedSpace string

	// FreeSpace is the container-wide free space with the
	// trailing byte-count stripped.
	FreeSpace string

	// Removable is true when diskutil reports
	// "Removable Media: Removable". Used by the keyboard to
	// show the Eject button.
	Removable bool

	// Internal is true when diskutil reports
	// "Device Location: Internal". Internal volumes don't get
	// the Eject button regardless of Removable.
	Internal bool

	// ReadOnly is true when diskutil reports
	// "Volume Read-Only: Yes…". Currently informational only;
	// no action gates on it.
	ReadOnly bool

	// SolidState is true when diskutil reports
	// "Solid State: Yes". Currently informational only.
	SolidState bool

	// Raw is the full `diskutil info` output, preserved for
	// fallback display when parsing missed key fields.
	Raw string
}

// DisksList returns the user-facing subset of mounted volumes:
// the root volume "/" and anything under "/Volumes/" (external
// drives, mounted DMGs, network mounts).
//
// Behavior:
//   - Shells out to `df -h` and walks the output line by line.
//   - Skips the header row.
//   - Skips lines with fewer than 9 whitespace-split fields
//     (defensive against malformed entries).
//   - Joins fields[8:] as the mount path so paths containing
//     spaces ("/Volumes/Foo Bar/") survive intact.
//   - Filters: keeps only mount == "/" OR mount starts with
//     "/Volumes/". Drops devfs, /System/Volumes/*, simulator
//     mounts, and other internal noise — meaningless for a
//     remote-control bot.
//
// Returns the filtered slice (may be empty if no qualifying
// mounts exist) or the underlying df error.
func (s *Service) DisksList(ctx context.Context) ([]DiskVolume, error) {
	out, err := s.r.Exec(ctx, "df", "-h")
	if err != nil {
		return nil, err
	}
	var volumes []DiskVolume
	for i, line := range strings.Split(string(out), "\n") {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		mount := strings.Join(fields[8:], " ")
		if mount != "/" && !strings.HasPrefix(mount, "/Volumes/") {
			continue
		}
		volumes = append(volumes, DiskVolume{
			Filesystem: fields[0],
			Size:       fields[1],
			Used:       fields[2],
			Available:  fields[3],
			Capacity:   fields[4],
			MountedOn:  mount,
		})
	}
	return volumes, nil
}

// diskutilSetters maps each diskutil KEY to a function that
// parses VALUE and writes it onto a [DiskDetails]. Built once at
// init so DiskInfo's inner loop is a single map lookup per line.
var diskutilSetters = map[string]func(*DiskDetails, string){
	"Volume Name":             func(d *DiskDetails, v string) { d.VolumeName = v },
	"Mount Point":             func(d *DiskDetails, v string) { d.MountPoint = v },
	"Device Node":             func(d *DiskDetails, v string) { d.Device = v },
	"File System Personality": func(d *DiskDetails, v string) { d.FSType = v },
	"Disk Size":               func(d *DiskDetails, v string) { d.DiskSize = trimAfterParen(v) },
	"Volume Used Space":       func(d *DiskDetails, v string) { d.UsedSpace = trimAfterParen(v) },
	"Container Free Space":    func(d *DiskDetails, v string) { d.FreeSpace = trimAfterParen(v) },
	"Removable Media":         func(d *DiskDetails, v string) { d.Removable = strings.EqualFold(v, "Removable") },
	"Device Location":         func(d *DiskDetails, v string) { d.Internal = strings.EqualFold(v, "Internal") },
	"Volume Read-Only":        func(d *DiskDetails, v string) { d.ReadOnly = strings.HasPrefix(strings.ToLower(v), "yes") },
	"Solid State":             func(d *DiskDetails, v string) { d.SolidState = strings.EqualFold(v, "Yes") },
}

// DiskInfo returns the parsed per-disk detail view via
// `diskutil info <mount>`.
//
// Behavior:
//   - Rejects empty mount with "mount is required".
//   - Shells out to `diskutil info <mount>`.
//   - On subprocess failure, returns DiskDetails{Raw: <stdout>}
//     plus the error so callers can still display the raw text.
//   - On success, walks the output line by line, splitting each
//     into KEY: VALUE via [splitDiskutilKV] and looking up KEY in
//     [diskutilSetters] to populate the typed field.
//   - Size strings (Disk Size / Volume Used Space / Container
//     Free Space) get stripped of diskutil's trailing
//     " (N Bytes) (exactly …)" suffix via [trimAfterParen].
//   - Bool flags use case-insensitive matches against the
//     diskutil-reported value: Removable Media: Removable,
//     Device Location: Internal, Volume Read-Only: Yes…,
//     Solid State: Yes.
//   - Fields not present in the diskutil output are left at
//     their zero value — partial parses are normal.
func (s *Service) DiskInfo(ctx context.Context, mount string) (DiskDetails, error) {
	if mount == "" {
		return DiskDetails{}, fmt.Errorf("mount is required")
	}
	out, err := s.r.Exec(ctx, "diskutil", "info", mount)
	if err != nil {
		return DiskDetails{Raw: string(out)}, err
	}
	d := DiskDetails{Raw: string(out)}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := splitDiskutilKV(line)
		if !ok {
			continue
		}
		if setter, ok := diskutilSetters[key]; ok {
			setter(&d, val)
		}
	}
	return d, nil
}

// EjectDisk runs `diskutil eject <mount>`. The keyboard layer
// gates this behind the [DiskDetails.Removable] check so the
// button only appears for actually-ejectable volumes — but this
// method is the trusted boundary; calling it on the root volume
// would fail at the diskutil layer.
//
// Behavior:
//   - Rejects empty mount with "mount is required".
//   - Returns the runner error verbatim on diskutil failure
//     ("Eject failed: Cannot eject because the volume is in
//     use" is a common one).
func (s *Service) EjectDisk(ctx context.Context, mount string) error {
	if mount == "" {
		return fmt.Errorf("mount is required")
	}
	_, err := s.r.Exec(ctx, "diskutil", "eject", mount)
	return err
}

// OpenInFinder runs `open <mount>` to reveal the volume in
// Finder. Works on every mount type — root, external, network,
// DMG.
//
// Behavior:
//   - Rejects empty mount with "mount is required".
//   - Returns the runner error verbatim on `open` failure (rare;
//     `open` is forgiving and will silently no-op on
//     not-actually-ejected paths).
func (s *Service) OpenInFinder(ctx context.Context, mount string) error {
	if mount == "" {
		return fmt.Errorf("mount is required")
	}
	_, err := s.r.Exec(ctx, "open", mount)
	return err
}

// splitDiskutilKV splits a `   KEY:    VALUE` diskutil line into
// the trimmed key + trimmed value.
//
// Behavior:
//   - Returns ok=false when line has no ':' separator.
//   - Returns ok=false when the trimmed key is empty (defends
//     against ": foo" garbage lines).
//   - Splits on the FIRST ':' so values containing ':' (rare in
//     diskutil but possible in device-path values) survive intact.
func splitDiskutilKV(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

// trimAfterParen strips diskutil's trailing
// " (N Bytes) (exactly N Bytes)" suffix from a size string,
// returning just the human form ("494.4 GB").
//
// Behavior:
//   - Searches for the first " (" and returns everything before
//     it (trimmed).
//   - On no match, returns the input trimmed.
//
// Used by [Service.DiskInfo] to clean up Disk Size / Volume Used
// Space / Container Free Space values for display.
func trimAfterParen(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
