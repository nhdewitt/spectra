//go:build linux || freebsd || darwin

package disk

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

func createDeviceToMountpointMap(mounts []MountInfo) map[string]MountInfo {
	deviceMap := make(map[string]MountInfo)
	for _, info := range mounts {
		deviceName := strings.TrimPrefix(info.Device, "/dev/")
		if _, exists := deviceMap[deviceName]; !exists {
			deviceMap[deviceName] = info
		}
	}
	return deviceMap
}

func RunMountManager(ctx context.Context, cache *DriveCache, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	updateCache(cache)

	for {
		select {
		case <-ticker.C:
			updateCache(cache)
		case <-ctx.Done():
			slog.Debug("mount manager stopped")
			return
		}
	}
}

func updateCache(cache *DriveCache) {
	currentMounts, err := parseMounts()
	if err != nil {
		slog.Warn("mount cache update failed, disk metrics will go stale", "error", err)
		return
	}

	newMap := createDeviceToMountpointMap(currentMounts)

	cache.RWMutex.Lock()
	cache.DeviceToMountpoint = newMap
	cache.RWMutex.Unlock()
}
