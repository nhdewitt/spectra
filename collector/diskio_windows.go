//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nhdewitt/spectra/metrics"
)

const (
	timeDelta              float64 = 5.0
	IOCTL_DISK_PERFORMANCE uint32  = 0x70020
)

type diskPerformance struct {
	BytesRead           int64
	BytesWritten        int64
	ReadTime            int64
	WriteTime           int64
	IdleTime            int64
	ReadCount           uint32
	WriteCount          uint32
	QueueDepth          uint32
	SplitCount          uint32
	QueryTime           int64
	StorageDeviceNumber uint32
	StorageManagerName  [8]uint16
}

// perfGetter allows mocking getDrivePerformance in tests
type perfGetter func(driveIndex uint32) (diskPerformance, error)

var (
	lastDiskPerf map[uint32]diskPerformance
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

	// Baseline
	if lastDiskPerf == nil {
		lastDiskPerf = currentPerf
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

		result = append(result, metrics.DiskIOMetric{
			Device:     deviceName,
			ReadBytes:  uint64(readBytesDelta / timeDelta),
			WriteBytes: uint64(writeBytesDelta / timeDelta),
			ReadOps:    uint64(float64(curr.ReadCount-prev.ReadCount) / timeDelta),
			WriteOps:   uint64(float64(curr.WriteCount-prev.WriteCount) / timeDelta),
			ReadTime:   uint64(curr.ReadTime - prev.ReadTime),
			WriteTime:  uint64(curr.WriteTime - prev.WriteTime),
			InProgress: uint64(curr.QueueDepth),
		})
	}

	lastDiskPerf = currentPerf
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
		IOCTL_DISK_PERFORMANCE,
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
