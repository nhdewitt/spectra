//go:build freebsd

package disk

import (
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
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
		UsedPct:     util.Percent(used, total),
		InodesTotal: stat.Files,
		InodesUsed:  inodesUsed,
		InodesPct:   util.Percent(inodesUsed, stat.Files),
	}
}
