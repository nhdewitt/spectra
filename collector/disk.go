package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nhdewitt/raspimon/metrics"
	"golang.org/x/sys/unix"
)

const bytesPerMB uint64 = 1024 * 1024

var ignoredFilesystems = map[string]struct{}{
	"proc":       {},
	"sysfs":      {},
	"devtmpfs":   {},
	"cgroup":     {},
	"securityfs": {},
	"tmpfs":      {},
	"ramfs":      {},
	"nfs":        {},
	"cifs":       {},
	"autofs":     {},
	"fuse":       {},
	"overlay":    {},
}

var localFilesystems = map[string]struct{}{
	"ext4":  {},
	"ext3":  {},
	"xfs":   {},
	"btrfs": {},
	"vfat":  {},
	"ntfs":  {},
}

type mountInfo struct {
	Device     string
	Mountpoint string
	FSType     string
}

func CollectDisk(ctx context.Context) ([]metrics.Metric, error) {
	mounts, err := parseMounts()
	if err != nil {
		return nil, fmt.Errorf("parsing mounts: %w", err)
	}

	result := make([]metrics.Metric, 0, len(mounts))

	for _, m := range mounts {
		stat, err := statfs(m.Mountpoint)
		if err != nil {
			continue
		}

		result = append(result, buildDiskMetric(m, stat))
	}

	return result, nil
}

func parseMounts() ([]mountInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mounts []mountInfo
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		m := mountInfo{
			Device:     fields[0],
			Mountpoint: fields[1],
			FSType:     fields[2],
		}

		if shouldIgnore(m) {
			continue
		}

		mounts = append(mounts, m)
	}

	return mounts, scanner.Err()
}

func shouldIgnore(m mountInfo) bool {
	if _, ignored := ignoredFilesystems[m.FSType]; ignored {
		return true
	}
	if strings.HasPrefix(m.Device, "/dev/loop") {
		return true
	}
	return false
}

func statfs(path string) (unix.Statfs_t, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	return stat, err
}

func buildDiskMetric(m mountInfo, stat unix.Statfs_t) metrics.DiskMetric {
	bsize := uint64(stat.Bsize)

	total := (stat.Blocks * bsize) / bytesPerMB
	available := (stat.Bavail * bsize) / bytesPerMB
	used := ((stat.Blocks - stat.Bfree) * bsize) / bytesPerMB
	inodesUsed := stat.Files - stat.Ffree

	return metrics.DiskMetric{
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
