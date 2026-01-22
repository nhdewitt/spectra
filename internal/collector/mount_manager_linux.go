//go:build !windows
// +build !windows

package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
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

func parseMounts() ([]MountInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseMountsFrom(f)
}

func parseMountsFrom(r io.Reader) ([]MountInfo, error) {
	var mounts []MountInfo
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		m := MountInfo{
			Device:     fields[0],
			Mountpoint: decodeMountPath(fields[1]),
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
	_, isFSTypeIgnored := ignoredFilesystems[m.FSType]

	return isFSTypeIgnored || strings.HasPrefix(m.Device, "/dev/loop") ||
		strings.HasPrefix(m.Mountpoint, "/mnt/wsl/") ||
		strings.HasPrefix(m.Mountpoint, "/Docker/")
}

// decodeMountPath replaces common octal escapes in /proc/mounts.
func decodeMountPath(s string) string {
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
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
