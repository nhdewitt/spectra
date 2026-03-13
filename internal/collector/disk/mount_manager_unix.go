//go:build linux || freebsd || darwin

package disk

import (
	"context"
	"fmt"
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
			fmt.Println("Mount manager stopped.")
			return
		}
	}
}

func updateCache(cache *DriveCache) {
	currentMounts, err := parseMounts()
	if err != nil {
		fmt.Printf("Error updating mount cache: %v\n", err)
		return
	}

	newMap := createDeviceToMountpointMap(currentMounts)

	cache.RWMutex.Lock()
	cache.DeviceToMountpoint = newMap
	cache.RWMutex.Unlock()
}
