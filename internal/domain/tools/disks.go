package tools

import (
	"context"
	"fmt"
	"strings"
)

// DiskVolume is a mounted filesystem (one row of `df -h`, parsed).
type DiskVolume struct {
	// Filesystem is the device path from `df`, e.g. "/dev/disk3s1s1".
	Filesystem string
	// Size is the human-readable total size column.
	Size string
	// Used is the human-readable used-bytes column.
	Used string
	// Available is the human-readable free-bytes column.
	Available string
	// Capacity is the percent-used column, e.g. "42%".
	Capacity string
	// MountedOn is the mount point, e.g. "/" or "/Volumes/Backup".
	MountedOn string
}

// DiskDetails is the per-disk drill-down view, parsed from
// `diskutil info <mount>`. Empty fields mean diskutil didn't expose
// that line on this Mac / for this volume.
type DiskDetails struct {
	// VolumeName is the user-visible volume name, e.g. "Macintosh HD".
	VolumeName string
	// MountPoint is the mount path, e.g. "/".
	MountPoint string
	// Device is the device node, e.g. "/dev/disk3s1s1".
	Device string
	// FSType is the filesystem personality reported by diskutil,
	// e.g. "APFS".
	FSType string
	// DiskSize is the total capacity string with trailing byte
	// counts stripped, e.g. "494.4 GB".
	DiskSize string
	// UsedSpace is the used portion of the APFS container, stripped
	// of the trailing byte count.
	UsedSpace string
	// FreeSpace is the container-wide free space, stripped of the
	// trailing byte count.
	FreeSpace string
	// Removable is true when diskutil reports "Removable Media:
	// Removable".
	Removable bool
	// Internal is true when diskutil reports "Device Location:
	// Internal".
	Internal bool
	// ReadOnly is true when diskutil reports "Volume Read-Only: Yes…".
	ReadOnly bool
	// SolidState is true when diskutil reports "Solid State: Yes".
	SolidState bool
	// Raw is the full `diskutil info` output for fallback display.
	Raw string
}

// DisksList returns user-facing mounts only: the root volume "/" and
// anything under "/Volumes/" (external drives, mounted DMGs). System
// mounts, devfs, simulator volumes, and other internal noise are
// hidden — they're meaningless for a remote-control bot.
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

// DiskInfo returns parsed detail for one mount via `diskutil info`.
// Best-effort: missing fields stay empty; Raw always carries the
// full output for fallback rendering.
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
		switch key {
		case "Volume Name":
			d.VolumeName = val
		case "Mount Point":
			d.MountPoint = val
		case "Device Node":
			d.Device = val
		case "File System Personality":
			d.FSType = val
		case "Disk Size":
			d.DiskSize = trimAfterParen(val)
		case "Volume Used Space":
			d.UsedSpace = trimAfterParen(val)
		case "Container Free Space":
			d.FreeSpace = trimAfterParen(val)
		case "Removable Media":
			d.Removable = strings.EqualFold(val, "Removable")
		case "Device Location":
			d.Internal = strings.EqualFold(val, "Internal")
		case "Volume Read-Only":
			d.ReadOnly = strings.HasPrefix(strings.ToLower(val), "yes")
		case "Solid State":
			d.SolidState = strings.EqualFold(val, "Yes")
		}
	}
	return d, nil
}

// EjectDisk runs `diskutil eject <mount>`. Caller is responsible for
// only invoking this on volumes that are actually ejectable
// (typically /Volumes/* with Removable Media: Removable).
func (s *Service) EjectDisk(ctx context.Context, mount string) error {
	if mount == "" {
		return fmt.Errorf("mount is required")
	}
	_, err := s.r.Exec(ctx, "diskutil", "eject", mount)
	return err
}

// OpenInFinder runs `open <mount>` to reveal the volume in Finder.
func (s *Service) OpenInFinder(ctx context.Context, mount string) error {
	if mount == "" {
		return fmt.Errorf("mount is required")
	}
	_, err := s.r.Exec(ctx, "open", mount)
	return err
}

// splitDiskutilKV parses a `   KEY:    VALUE` line from diskutil
// output. Returns ok=false on lines without ':' or with empty key.
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

// trimAfterParen strips the long " (N Bytes) (exactly …)" suffix
// diskutil tacks onto size lines, leaving just the human form
// ("494.4 GB").
func trimAfterParen(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
