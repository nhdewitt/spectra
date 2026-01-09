//go:build windows
// +build windows

package collector

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

func RunMountManager(ctx context.Context, cache *DriveCache, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	updateDriveCacheNative(cache)

	for {
		select {
		case <-ticker.C:
			updateDriveCacheNative(cache)
		case <-ctx.Done():
			log.Println("Mount Manager stopped.")
			return
		}
	}
}

func updateDriveCacheNative(cache *DriveCache) {
	allDrives := scanPhysicalDrives()

	allowedMap := make(map[uint32]MountInfo)
	for _, d := range allDrives {
		if d.InterfaceType == BusTypeUsb || d.InterfaceType == BusType1394 {
			continue
		}

		if strings.Contains(strings.ToLower(d.Model), "virtual") {
			continue
		}

		allowedMap[d.Index] = d
	}

	letterMap := mapDriveLettersToPhysicalDisks(allowedMap)

	cache.RWMutex.Lock()
	defer cache.RWMutex.Unlock()

	cache.AllowedDrives = allowedMap
	cache.DriveLetterMap = letterMap
}

func scanPhysicalDrives() []MountInfo {
	var drives []MountInfo

	for i := uint32(0); i < 64; i++ {
		path := fmt.Sprintf(`\\.\PhysicalDrive%d`, i)
		pathPtr, _ := windows.UTF16PtrFromString(path)

		handle, err := windows.CreateFile(
			pathPtr,
			0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err != nil {
			continue
		}

		info, err := getStorageProperty(handle, i)
		windows.CloseHandle(handle)

		if err == nil {
			drives = append(drives, info)
		}
	}

	return drives
}

func getStorageProperty(handle windows.Handle, index uint32) (MountInfo, error) {
	var query storagePropertyQuery
	query.PropertyId = storageDeviceProperty
	query.QueryType = propertyStandardQuery

	buf := make([]byte, 1024)
	var bytesReturned uint32

	err := windows.DeviceIoControl(
		handle,
		ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&query)),
		uint32(unsafe.Sizeof(query)),
		(*byte)(unsafe.Pointer(&buf[0])),
		uint32(len(buf)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return MountInfo{}, err
	}

	header := (*storageDeviceDescriptor)(unsafe.Pointer(&buf[0]))

	model := ""
	if header.ProductIdOffset > 0 && header.ProductIdOffset < bytesReturned {
		model = extractString(buf, header.ProductIdOffset)
	}

	model = strings.TrimSpace(model)
	if model == "" {
		model = fmt.Sprintf("PhysicalDrive%d", index)
	}

	return MountInfo{
		DeviceID:      fmt.Sprintf(`\\.\PHYSICALDRIVE%d`, index),
		Index:         index,
		Model:         model,
		InterfaceType: BusType(header.BusType),
	}, nil
}

// extractString reads a null-terminated string from a raw buffer at offset
func extractString(buf []byte, offset uint32) string {
	if offset >= uint32(len(buf)) {
		return ""
	}

	end := bytes.IndexByte(buf[offset:], 0)
	if end == -1 {
		return string(buf[offset:])
	}

	return string(buf[offset : offset+uint32(end)])
}

func mapDriveLettersToPhysicalDisks(physicalDrives map[uint32]MountInfo) map[uint32][]string {
	result := make(map[uint32][]string)

	ret, _, _ := procGetLogicalDrives.Call()
	mask := uint32(ret)

	for i := range 26 {
		if mask&(1<<i) == 0 {
			continue
		}
		letter := string(rune('A'+i)) + ":"
		diskNum, err := getPhysicalDiskNumber(letter)
		if err != nil {
			continue
		}

		if _, ok := physicalDrives[diskNum]; ok {
			result[diskNum] = append(result[diskNum], letter)
		}
	}

	return result
}

func getPhysicalDiskNumber(driveLetter string) (uint32, error) {
	path := fmt.Sprintf(`\\.\%s`, driveLetter)
	pathPtr, _ := windows.UTF16PtrFromString(path)

	handle, err := windows.CreateFile(
		pathPtr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, nil
	}
	defer windows.CloseHandle(handle)

	var extents volumeDiskExtents
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		handle,
		ioctlVolumeGetVolumeDiskExtents,
		nil,
		0,
		(*byte)(unsafe.Pointer(&extents)),
		uint32(unsafe.Sizeof(extents)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, nil
	}

	if extents.NumberOfDiskExtents > 0 {
		return extents.Extents[0].DiskNumber, nil
	}

	return 0, fmt.Errorf("no extents found")
}

func MakeDiskCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return CollectDisk(ctx)
	}
}

func MakeDiskIOCollector(cache *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return CollectDiskIO(ctx, cache)
	}
}
