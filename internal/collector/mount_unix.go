//go:build !windows

package collector

import "sync"

type MountInfo struct {
	Device     string
	Mountpoint string
	FSType     string
}

type DriveCache struct {
	sync.RWMutex
	DeviceToMountpoint map[string]MountInfo
}

func NewDriveCache() *DriveCache {
	return &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}
}

// GetDefaultPath returns "/" if present, or the first available mount
func (c *DriveCache) GetDefaultPath() string {
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.DeviceToMountpoint["/"]; ok {
		return "/"
	}

	for _, info := range c.DeviceToMountpoint {
		return info.Mountpoint
	}

	return "."
}
