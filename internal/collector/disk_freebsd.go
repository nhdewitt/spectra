//go:build freebsd

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

var ignoredFilesystems = map[string]struct{}{
	// Pseudo/virtual filesystems
	"devfs":     {}, // Device nodes
	"fdescfs":   {}, // File descriptors
	"procfs":    {}, // Process info
	"linprocfs": {}, // Process info (Linux-compatible)
	"linsysfs":  {}, // sysfs (Linux-compatible)
	"tmpfs":     {}, // In-memory tmp
	"nullfs":    {}, // Loopback/bind mount
	"unionfs":   {}, // Union mount
	"autofs":    {}, // Automounter
	"fusefs":    {}, // Generic FUSE

	// Network - prevent statfs hangs if down
	"nfs":   {},
	"smbfs": {},
}

var localFilesystems = map[string]struct{}{
	// FreeBSD Native
	"ufs": {}, // Unix File System
	"zfs": {}, // Z file system

	// Compatibility/Other
	"ext2fs":  {}, // Ext2/3/4
	"msdosfs": {}, // FAT/EFI
	"ntfs":    {}, // NTFS
	"cd9660":  {}, // ISO/CD-ROM
	"udf":     {}, // DVD/Optical
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

func statfs(path string) (unix.Statfs_t, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	return stat, err
}

func loadMountMap(cache *DriveCache) map[string]MountInfo {
	cache.RWMutex.RLock()
	defer cache.RWMutex.RUnlock()

	out := make(map[string]MountInfo, len(cache.DeviceToMountpoint))
	for k, v := range cache.DeviceToMountpoint {
		out[k] = v
	}

	return out
}

func buildDiskMetric(m MountInfo, stat unix.Statfs_t) protocol.DiskMetric {
	bsize := uint64(stat.Bsize)

	// Bavail and Ffree are int64 on FreeBSD,
	// cast to uint64 and clamp to 0.
	stat.Bavail = max(stat.Bavail, 0)
	bavail := uint64(stat.Bavail)
	stat.Ffree = max(stat.Ffree, 0)
	ffree := uint64(stat.Ffree)
	bfree := stat.Bfree

	// Underflow guards
	if bfree > stat.Blocks {
		bfree = stat.Blocks
	}
	if ffree > stat.Files {
		ffree = stat.Files
	}

	total := stat.Blocks * bsize
	available := bavail * bsize
	used := (stat.Blocks - bfree) * bsize
	inodesUsed := stat.Files - ffree

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
