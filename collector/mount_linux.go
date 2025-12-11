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
