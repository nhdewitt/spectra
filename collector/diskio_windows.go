//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/metrics"
)

// perfGetter allows mocking getDrivePerformance in tests
type perfGetter func(driveIndex uint32) (diskPerformance, error)

var (
	lastDiskPerf map[uint32]diskPerformance
	lastDiskTime time.Time
	// getDrivePerf is the function used to get performance data (mockable)
	getDrivePerf perfGetter = getDrivePerformance
)

func CollectDiskIO(ctx context.Context, driveCache *DriveCache) ([]metrics.Metric, error) {
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

	secondsElapsed := now.Sub(lastDiskTime).Seconds()
	if secondsElapsed <= 0 {
		log.Printf("Warning: Invalid time delta (%f seconds). Now: %v, Last: %v", secondsElapsed, now, lastDiskTime)
		lastDiskPerf = currentPerf
		lastDiskTime = now
		return nil, nil
	}

	result := make([]metrics.Metric, 0, len(currentPerf))

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

		result = append(result, metrics.DiskIOMetric{
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
	pathPtr, _ := syscall.UTF16PtrFromString(path)

	handle, err := syscall.CreateFile(
		pathPtr,
		0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return diskPerformance{}, fmt.Errorf("CreateFile failed: %w", err)
	}
	defer syscall.CloseHandle(handle)

	var perf diskPerformance
	var bytesReturned uint32

	err = syscall.DeviceIoControl(
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

func formatDeviceName(idx uint32, driveInfo Win32_DiskDrive, letterMap map[uint32][]string) string {
	if letters, ok := letterMap[idx]; ok && len(letters) > 0 {
		return strings.Join(letters, ", ")
	}
	if driveInfo.Model != "" {
		return driveInfo.Model
	}
	return fmt.Sprintf("PhysicalDrive%d", idx)
}
