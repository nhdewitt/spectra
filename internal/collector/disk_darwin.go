//go:build darwin

package collector

import (
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

var ignoredFilesystems = map[string]struct{}{
	"devfs":   {},
	"autofs":  {},
	"nullfs":  {},
	"unionfs": {},
	"smbfs":   {},
	"nfs":     {},
	"afpfs":   {},
	"vmhgfs":  {},
	"ftp":     {},
}

var localFilesystems = map[string]struct{}{
	"apfs":    {},
	"hfs":     {},
	"hfsplus": {},
	"ufs":     {},
	"exfat":   {},
	"msdos":   {},
	"ntfs":    {},
}

var ignoredMounts = map[string]struct{}{
	"/System/Volumes/VM":         {},
	"/System/Volumes/Preboot":    {},
	"/System/Volumes/Update":     {},
	"/System/Volumes/xarts":      {},
	"/System/Volumes/iSCPreboot": {},
	"/System/Volumes/Hardware":   {},
}

func parseMounts() ([]MountInfo, error) {
	// Call with nil first to get count
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	buf := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	var mounts []MountInfo
	for _, fs := range buf[:n] {
		m := MountInfo{
			Device:     charsToString(fs.Mntfromname[:]),
			Mountpoint: charsToString(fs.Mntonname[:]),
			FSType:     charsToString(fs.Fstypename[:]),
		}

		if shouldIgnore(m) {
			continue
		}

		mounts = append(mounts, m)
	}

	return mounts, nil
}

func shouldIgnore(m MountInfo) bool {
	_, ignoredFs := ignoredFilesystems[m.FSType]
	_, ignoredMnt := ignoredMounts[m.Mountpoint]
	return ignoredFs || ignoredMnt || strings.HasPrefix(m.Device, "map ")
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
