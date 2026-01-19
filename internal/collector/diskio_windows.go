//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

// perfGetter allows mocking getDrivePerformance in tests
type perfGetter func(driveIndex uint32) (diskPerformance, error)

var (
	lastDiskPerf map[uint32]diskPerformance
	lastDiskTime time.Time
	// getDrivePerf is the function used to get performance data (mockable)
	getDrivePerf perfGetter = getDrivePerformance
)

func CollectDiskIO(ctx context.Context, driveCache *DriveCache) ([]protocol.Metric, error) {
	driveCache.RLock()
	allowedDrives := driveCache.AllowedDrives
	letterMap := driveCache.DriveLetterMap
	driveCache.RUnlock()

	if len(allowedDrives) == 0 {
		return nil, nil
	}

	currentPerf := make(map[uint32]diskPerformance)

	for idx, driveInfo := range allowedDrives {
		perf, err := getDrivePerf(idx)
		if err != nil {
			log.Printf("Unable to get IO performance for %s: %v", driveInfo.Model, err)
			continue
		}
		currentPerf[idx] = perf
	}

	now := time.Now()

	// Baseline
	if lastDiskPerf == nil {
		lastDiskPerf = currentPerf
		lastDiskTime = now
		return nil, nil
	}

	// Time Delta Calculation
	secondsElapsed := validateTimeDelta(now, lastDiskTime, "disk_io")
	if secondsElapsed == 0 {
		lastDiskPerf = currentPerf
		lastDiskTime = now
		return nil, nil
	}

	result := make([]protocol.Metric, 0, len(currentPerf))

	for idx, curr := range currentPerf {
		prev, ok := lastDiskPerf[idx]
		if !ok {
			continue
		}

		driveInfo := allowedDrives[idx]
		deviceName := formatDeviceName(idx, driveInfo, letterMap)

		readBytesDelta := float64(curr.BytesRead - prev.BytesRead)
		writeBytesDelta := float64(curr.BytesWritten - prev.BytesWritten)
		readOpsDelta := float64(curr.ReadCount - prev.ReadCount)
		writeOpsDelta := float64(curr.WriteCount - prev.WriteCount)

		readTimeDelta := uint64(curr.ReadTime - prev.ReadTime)
		writeTimeDelta := uint64(curr.WriteTime - prev.WriteTime)

		result = append(result, protocol.DiskIOMetric{
			Device:     deviceName,
			ReadBytes:  uint64(readBytesDelta / secondsElapsed),
			WriteBytes: uint64(writeBytesDelta / secondsElapsed),
			ReadOps:    uint64(readOpsDelta / secondsElapsed),
			WriteOps:   uint64(writeOpsDelta / secondsElapsed),
			ReadTime:   readTimeDelta,
			WriteTime:  writeTimeDelta,
			InProgress: uint64(curr.QueueDepth),
		})
	}

	lastDiskPerf = currentPerf
	lastDiskTime = now
	return result, nil
}

func getDrivePerformance(driveIndex uint32) (diskPerformance, error) {
	path := fmt.Sprintf(`\\.\PhysicalDrive%d`, driveIndex)
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
		return diskPerformance{}, fmt.Errorf("CreateFile failed: %w", err)
	}
	defer windows.CloseHandle(handle)

	var perf diskPerformance
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		handle,
		ioctlDiskPerformance,
		nil,
		0,
		(*byte)(unsafe.Pointer(&perf)),
		uint32(unsafe.Sizeof(perf)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return diskPerformance{}, fmt.Errorf("DeviceIoControl failed: %w", err)
	}

	return perf, nil
}

func formatDeviceName(idx uint32, driveInfo MountInfo, letterMap map[uint32][]string) string {
	letters := ""
	if l, ok := letterMap[idx]; ok && len(l) > 0 {
		letters = strings.Join(l, ", ")
	}

	if driveInfo.Model != "" {
		if letters != "" {
			return fmt.Sprintf("%s (%s)", driveInfo.Model, letters)
		}
		return driveInfo.Model
	}

	if letters != "" {
		return letters
	}
	return fmt.Sprintf("PhysicalDrive%d", idx)
}
