//go:build windows

package collector

import (
	"sync"
)

// BusType represents the hardware interface (USB, SATA, NVMe, etc.)
type BusType uint32

// DiskInfo represents a phsyical disk detected on the system
type MountInfo struct {
	DeviceID      string // "\\.\PHYSICALDRIVE0"
	Index         uint32
	Model         string  // "Samsung SSD 970 EVO"
	InterfaceType BusType // BusTypeNvme (17)
}

type DriveCache struct {
	sync.RWMutex
	AllowedDrives  map[uint32]MountInfo
	DriveLetterMap map[uint32][]string // ["C:", "D:", ...]
}

func NewDriveCache() *DriveCache {
	return &DriveCache{
		AllowedDrives:  make(map[uint32]MountInfo),
		DriveLetterMap: make(map[uint32][]string),
	}
}

// GetDefaultPath returns "C:" if present, or the first available drive letter
func (c *DriveCache) GetDefaultPath() string {
	c.RLock()
	defer c.RUnlock()

	for _, letters := range c.DriveLetterMap {
		for _, letter := range letters {
			if letter == "C:" {
				return "C:\\"
			}
		}
	}

	for _, letters := range c.DriveLetterMap {
		if len(letters) > 0 {
			return letters[0] + "\\"
		}
	}

	// Fail-safe: Current Directory
	return "."
}
