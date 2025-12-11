//go:build windows

package collector

import (
	"sync"
)

type Win32_DiskDrive struct {
	DeviceID      string
	InterfaceType string
	MediaType     string
	Model         string
	Index         uint32
}

// Win32_DiskDriveToDiskPartition associates physical drives to partitions
type Win32_DiskDriveToDiskPartition struct {
	Antecedent string // PhysicalDrive reference
	Dependent  string // Partition reference
}

// Win32_LogicalDiskToPartition associates partitions to drive letters
type Win32_LogicalDiskToPartition struct {
	Antecedent string // Partition reference
	Dependent  string // LogicalDisk reference (includes drive letter)
}

type DriveCache struct {
	sync.RWMutex
	AllowedDrives  map[uint32]Win32_DiskDrive
	DriveLetterMap map[uint32][]string // ["C:", "D:", ...]
}

func NewDriveCache() *DriveCache {
	return &DriveCache{
		AllowedDrives:  make(map[uint32]Win32_DiskDrive),
		DriveLetterMap: make(map[uint32][]string),
	}
}
