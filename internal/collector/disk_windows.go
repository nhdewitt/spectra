//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

var monitoredFilesystems = map[string]struct{}{
	"NTFS":  {},
	"FAT32": {},
	"FAT":   {},
	"EXFAT": {},
	"REFS":  {},
}

func CollectDisk(ctx context.Context) ([]protocol.Metric, error) {
	// Get bitmask of all available drives
	ret, _, _ := procGetLogicalDrives.Call()
	driveMask := uint32(ret)

	if driveMask == 0 {
		return nil, fmt.Errorf("GetLogicalDrives failed")
	}

	var result []protocol.Metric

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
