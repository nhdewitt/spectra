//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"math/bits"
	"strings"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

var monitoredFilesystems = map[string]struct{}{
	// Windows native
	"NTFS": {},
	"REFS": {}, // Resilient File System (Windows Server)

	// FAT
	"FAT32": {},
	"FAT":   {},
	"EXFAT": {},
}

func CollectDisk(ctx context.Context) ([]protocol.Metric, error) {
	// Get bitmask of all available drives
	ret, _, _ := procGetLogicalDrives.Call()
	driveMask := uint32(ret)

	if driveMask == 0 {
		return nil, fmt.Errorf("GetLogicalDrives failed")
	}

	// Optimize result to actual drive count return from GetLogicalDrives
	result := make([]protocol.Metric, 0, bits.OnesCount32(driveMask))

	for i := range 26 {
		if driveMask&(1<<i) == 0 {
			continue
		}

		// Construct path
		rootPath := string(rune('A'+i)) + ":\\"
		rootPathPtr, _ := windows.UTF16PtrFromString(rootPath)

		// Check drive type - only fixed+removable
		typeRet, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(rootPathPtr)))
		driveType := uint32(typeRet)

		if driveType != driveFixed && driveType != driveRemovable {
			continue
		}

		// Get fs name
		var volNameBuf [256]uint16
		var fsNameBuf [256]uint16

		ret, _, _ := procGetVolumeInformation.Call(
			uintptr(unsafe.Pointer(rootPathPtr)),
			uintptr(unsafe.Pointer(&volNameBuf[0])),
			uintptr(len(volNameBuf)),
			0,
			0,
			0,
			uintptr(unsafe.Pointer(&fsNameBuf[0])),
			uintptr(len(fsNameBuf)),
		)
		if ret == 0 {
			continue
		}

		fsName := strings.ToUpper(windows.UTF16ToString(fsNameBuf[:]))

		if _, ok := monitoredFilesystems[fsName]; !ok {
			continue
		}

		var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

		ret, _, _ = procGetDiskFreeSpaceEx.Call(
			uintptr(unsafe.Pointer(rootPathPtr)),
			uintptr(unsafe.Pointer(&freeBytesAvailable)),
			uintptr(unsafe.Pointer(&totalNumberOfBytes)),
			uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
		)
		if ret == 0 {
			log.Printf("Warning: Failed to get space for %s", rootPath)
			continue
		}

		usedBytes := totalNumberOfBytes - freeBytesAvailable

		// Get Volume Label
		volLabel := windows.UTF16ToString(volNameBuf[:])
		deviceName := volLabel
		if deviceName == "" {
			deviceName = strings.TrimSuffix(rootPath, "\\") // Fallback
		}

		result = append(result, protocol.DiskMetric{
			Device:     deviceName,
			Mountpoint: rootPath,
			Filesystem: fsName,
			Type:       "local",
			Total:      totalNumberOfBytes,
			Used:       usedBytes,
			Available:  freeBytesAvailable,
			UsedPct:    percent(usedBytes, totalNumberOfBytes),
		})
	}

	return result, nil
}

// ListMounts flattens the DriveLetterMap into a list of generic mounts.
func (c *DriveCache) ListMounts() []protocol.MountInfo {
	c.RLock()
	defer c.RUnlock()

	var results []protocol.MountInfo

	// Physical Disks (Index -> MountInfo)
	for idx, diskInfo := range c.AllowedDrives {
		letters, ok := c.DriveLetterMap[idx]
		if !ok {
			continue
		}

		for _, letter := range letters {
			results = append(results, protocol.MountInfo{
				Mountpoint: letter,
				Device:     diskInfo.Model,
				FSType:     "NTFS",
			})
		}
	}

	return results
}
