//go:build !windows
// +build !windows

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

// ignoredFilesystems are virtual, network, or special-purpose filesystems
// that shouldn't appear in disk metrics
var ignoredFilesystems = map[string]struct{}{
	// Kernel/system virtual filesystems
	"proc":        {}, // Process information
	"sysfs":       {}, // Kernel object attributes
	"devtmpfs":    {}, // Device nodes
	"devpts":      {}, // Pseudo-terminal devices
	"tmpfs":       {}, // RAM-backed temp storage
	"ramfs":       {}, // Older RAM filesystem
	"rootfs":      {}, // Initial root filesystem
	"debugfs":     {}, // Kernel debugging
	"tracefs":     {}, // Kernel tracing
	"securityfs":  {}, // Security modules
	"configfs":    {}, // Kernel config
	"fusectl":     {}, // FUSE control
	"mqueue":      {}, // POSIX message queues
	"hugetlbfs":   {}, // Huge pages
	"binfmt_misc": {}, // Binary format handlers
	"pstore":      {}, // Persistent storage for panic logs
	"efivarfs":    {}, // EFI variables

	// Control groups
	"cgroup":  {},
	"cgroup2": {},

	// Security
	"selinuxfs": {}, // SELinux

	// BPF/namespaces
	"bpf":  {},
	"nsfs": {},

	// Network filesystems
	"nfs":        {}, // Network File System
	"nfs4":       {}, // NFS v4
	"nfsd":       {}, // NFS server
	"cifs":       {}, // Windows SMB share
	"smbfs":      {}, // Older SMB filesystem
	"9p":         {}, // Plan 9
	"rpc_pipefs": {}, // NFS RPC pipes
	"sunrpc":     {}, // Sun RPC

	// FUSE/virtual
	"fuse":            {}, // Generic FUSE
	"fuse.gvfsd-fuse": {}, // GNOME virtual filesystem
	"fuse.sshfs":      {}, // SSH filesystem
	"autofs":          {}, // Automounter

	// Container/overlay
	"overlay":  {}, // Container layered filesystem
	"squashfs": {}, // Read-only compressed

	// Optical media
	"iso9660": {},
	"udf":     {},
}

// localFilesystems are physical/local disk filesystems that should be monitored.
var localFilesystems = map[string]struct{}{
	// Linux native
	"ext2":     {}, // Legacy (still used for /boot)
	"ext3":     {}, // Journaled ext2
	"ext4":     {},
	"xfs":      {}, // Default on RHEL/CentOS
	"btrfs":    {}, // Copy-on-write, snapshots support
	"zfs":      {}, // Advanced CoW filesystem
	"bcachefs": {}, // Modern CoW filesystem, merged in kernel 6.7

	// Flash-optimized
	"f2fs": {}, // Flash-Friendly File System, SSDs+SD cards

	// Windows/cross-platform
	"ntfs":  {}, // Windows NTFS (via ntfs-3g)
	"vfat":  {}, // FAT32 (common for /boot/efi and USB drives)
	"exfat": {}, // Large USB drives and SD cards

	// macOS
	"hfsplus": {}, // macOS Extended, mounted on Linux
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

// ListMounts returns a generic list of mount points for the protocol.
func (c *DriveCache) ListMounts() []protocol.MountInfo {
	c.RLock()
	defer c.RUnlock()

	results := make([]protocol.MountInfo, 0, len(c.DeviceToMountpoint))

	for _, info := range c.DeviceToMountpoint {
		results = append(results, protocol.MountInfo{
			Mountpoint: info.Mountpoint,
			Device:     info.Device,
			FSType:     info.FSType,
		})
	}

	return results
}
