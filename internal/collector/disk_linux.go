//go:build !windows
// +build !windows

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

const bytesPerMB uint64 = 1024 * 1024

var ignoredFilesystems = map[string]struct{}{
	"proc":        {},
	"sysfs":       {},
	"devtmpfs":    {},
	"cgroup":      {},
	"securityfs":  {},
	"tmpfs":       {},
	"ramfs":       {},
	"nfs":         {},
	"cifs":        {},
	"autofs":      {},
	"fuse":        {},
	"overlay":     {},
	"debugfs":     {},
	"fusectl":     {},
	"binfmt_misc": {},
	"cgroup2":     {},
	"tracefs":     {},
	"devpts":      {},
	"hugetlbfs":   {},
	"configfs":    {},
	"mqueue":      {},
	"rootfs":      {},
	"9p":          {},
}

var localFilesystems = map[string]struct{}{
	"ext4":  {},
	"ext3":  {},
	"xfs":   {},
	"btrfs": {},
	"vfat":  {},
	"ntfs":  {},
}

func MakeDiskCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return CollectDisk(ctx, cache)
	}
}

func CollectDisk(ctx context.Context, cache *DriveCache) ([]protocol.Metric, error) {
	mountMap := loadMountMap(cache)

	result := make([]protocol.Metric, 0, len(mountMap))

	for _, m := range mountMap {
		stat, err := statfs(m.Mountpoint)
		if err != nil {
			continue
		}

		result = append(result, buildDiskMetric(m, stat))
	}

	return result, nil
}

func loadMountMap(cache *DriveCache) map[string]MountInfo {
	cache.RWMutex.RLock()
	mountMap := cache.DeviceToMountpoint
	cache.RWMutex.RUnlock()

	return mountMap
}

func statfs(path string) (unix.Statfs_t, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	return stat, err
}

func buildDiskMetric(m MountInfo, stat unix.Statfs_t) protocol.DiskMetric {
	bsize := uint64(stat.Bsize)

	total := stat.Blocks * bsize
	available := stat.Bavail * bsize
	used := (stat.Blocks - stat.Bfree) * bsize
	inodesUsed := stat.Files - stat.Ffree

	return protocol.DiskMetric{
		Device:      m.Device,
		Mountpoint:  m.Mountpoint,
		Filesystem:  m.FSType,
		Type:        fsCategory(m.FSType),
		Total:       total,
		Used:        used,
		Available:   available,
		UsedPct:     percent(used, total),
		InodesTotal: stat.Files,
		InodesUsed:  inodesUsed,
		InodesPct:   percent(inodesUsed, stat.Files),
	}
}

func fsCategory(fsType string) string {
	if _, local := localFilesystems[fsType]; local {
		return "local"
	}
	return "other"
}
