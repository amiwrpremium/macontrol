package tools

import (
	"context"
	"strings"
)

// DiskVolume is a mounted filesystem.
type DiskVolume struct {
	Filesystem string
	Size       string
	Used       string
	Available  string
	Capacity   string
	MountedOn  string
}

// DisksList returns an approximation of `df -h` rows filtered to user-facing
// volumes (no devfs, no system mounts).
func (s *Service) DisksList(ctx context.Context) ([]DiskVolume, error) {
	out, err := s.r.Exec(ctx, "df", "-h")
	if err != nil {
		return nil, err
	}
	var volumes []DiskVolume
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		mount := strings.Join(fields[8:], " ")
		if !strings.HasPrefix(mount, "/") || strings.HasPrefix(mount, "/System/Volumes/VM") ||
			strings.HasPrefix(mount, "/private") {
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
