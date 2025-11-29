package collector

import "sync"

type MountMap struct {
	sync.RWMutex
	DeviceToMountpoint map[string]MountInfo
}

type MountInfo struct {
	Device     string
	Mountpoint string
	FSType     string
}
