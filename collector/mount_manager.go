package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

func createDeviceToMountpointMap(mounts []MountInfo) map[string]MountInfo {
	deviceMap := make(map[string]MountInfo)

	for _, info := range mounts {
		deviceName := strings.TrimPrefix(info.Device, "/dev/")
		deviceMap[deviceName] = info
	}
	return deviceMap
}

func parseMounts() ([]MountInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mounts []MountInfo
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		m := MountInfo{
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

func shouldIgnore(m MountInfo) bool {
	if _, ignored := ignoredFilesystems[m.FSType]; ignored {
		return true
	}
	if strings.HasPrefix(m.Device, "/dev/loop") {
		return true
	}
	return false
}

func RunMountManager(ctx context.Context, cache *MountMap, interval time.Duration) {
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

func updateCache(cache *MountMap) {
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
