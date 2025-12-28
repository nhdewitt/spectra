//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/nhdewitt/spectra/metrics"
	"github.com/shirou/gopsutil/disk"
)

var monitoredFilesystems = map[string]struct{}{
	"NTFS":  {},
	"FAT32": {},
	"FAT":   {},
	"EXFAT": {},
	"REFS":  {},
}

func CollectDisk(ctx context.Context) ([]metrics.Metric, error) {
	// Get all partitions
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	result := make([]metrics.Metric, 0, len(partitions))

	// Iterate through partitions and collect usage
	for _, p := range partitions {
		// Ignore network paths and temporary filesystems
		if strings.HasPrefix(p.Mountpoint, "\\\\") || p.Fstype == "CDFS" || p.Fstype == "UDF" {
			continue
		}

		fsTypeUpper := strings.ToUpper(p.Fstype)
		if _, ok := monitoredFilesystems[fsTypeUpper]; !ok {
			continue
		}

		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			log.Printf("Warning: failed to get usage for %s: %v", p.Mountpoint, err)
		}

		result = append(result, metrics.DiskMetric{
			Device:     p.Device,
			Mountpoint: p.Mountpoint,
			Filesystem: p.Fstype,
			Type:       "local",
			Total:      usage.Total,
			Used:       usage.Used,
			Available:  usage.Free,
			UsedPct:    percent(usage.Used, usage.Total),
		})
	}

	return result, nil
}
