//go:build windows

package collector

import (
	"sync"
)

// BusType represents the hardware interface (USB, SATA, NVMe, etc.)
type BusType uint32

// DiskInfo represents a phsyical disk detected on the system
type DiskInfo struct {
	DeviceID      string // "\\.\PHYSICALDRIVE0"
	Index         uint32
	Model         string  // "Samsung SSD 970 EVO"
	InterfaceType BusType // BusTypeNvme (17)
}

type DriveCache struct {
	sync.RWMutex
	AllowedDrives  map[uint32]DiskInfo
	DriveLetterMap map[uint32][]string // ["C:", "D:", ...]
}

func NewDriveCache() *DriveCache {
	return &DriveCache{
		AllowedDrives:  make(map[uint32]DiskInfo),
		DriveLetterMap: make(map[uint32][]string),
	}
}
