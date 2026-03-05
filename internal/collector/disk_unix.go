//go:build linux || freebsd || darwin

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

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

func fsCategory(fsType string) string {
	if _, local := localFilesystems[fsType]; local {
		return "local"
	}
	return "other"
}

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
