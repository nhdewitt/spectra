//go:build linux

package collector

import (
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
