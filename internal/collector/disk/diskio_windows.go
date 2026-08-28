//go:build windows
// +build windows

package disk

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
	"github.com/nhdewitt/spectra/internal/winapi"
	"golang.org/x/sys/windows"
)

// DISK_PERFORMANCE counters are expressed in 100ns ticks.
const ticksPerMillisecond = 10_000

// perfGetter allows mocking getDrivePerformance in tests
type perfGetter func(driveIndex uint32) (winapi.DiskPerformance, error)

var (
	lastDiskPerf map[uint32]winapi.DiskPerformance
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

	currentPerf := make(map[uint32]winapi.DiskPerformance)

	for idx, driveInfo := range allowedDrives {
		perf, err := getDrivePerf(idx)
		if err != nil {
			log.Printf("Unable to get IO performance for %s: %v", driveInfo.Model, err)
			continue
		}
		currentPerf[idx] = perf
	}

	now := nowFunc()

	// Baseline
	if lastDiskPerf == nil {
		lastDiskPerf = currentPerf
		lastDiskTime = now
		return nil, nil
	}

	// Time Delta Calculation
	secondsElapsed := util.ValidateTimeDelta(now, lastDiskTime, "disk_io")
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

		readBytesDelta := float64(util.Delta(uint64(curr.BytesRead), uint64(prev.BytesRead)))
		writeBytesDelta := float64(util.Delta(uint64(curr.BytesWritten), uint64(prev.BytesWritten)))

		readOpsDelta := util.Delta(uint64(curr.ReadCount), uint64(prev.ReadCount))
		writeOpsDelta := util.Delta(uint64(curr.WriteCount), uint64(prev.WriteCount))

		// DISK_PERFORMANCE.ReadTime is a LARGE_INTEGER of 100ns ticks, not ms. It
		// was previously passed through unconverted, so Windows agents reported
		// values 10000x larger than Linux agents into the same column. util.Delta
		// clamps counter resets, which bare uint64 subtraction on an int64 counter
		// did not.
		readTimeDelta := util.Delta(uint64(curr.ReadTime), uint64(prev.ReadTime)) / ticksPerMillisecond
		writeTimeDelta := util.Delta(uint64(curr.WriteTime), uint64(prev.WriteTime)) / ticksPerMillisecond

		readLatency := AwaitMs(readTimeDelta, readOpsDelta)
		writeLatency := AwaitMs(writeTimeDelta, writeOpsDelta)
		readBusy := BusyPct(readTimeDelta, secondsElapsed)
		writeBusy := BusyPct(writeTimeDelta, secondsElapsed)

		result = append(result, protocol.DiskIOMetric{
			Device:     deviceName,
			ReadBytes:  uint64(readBytesDelta / secondsElapsed),
			WriteBytes: uint64(writeBytesDelta / secondsElapsed),
			ReadOps:    util.Rate(readOpsDelta, secondsElapsed),
			WriteOps:   util.Rate(writeOpsDelta, secondsElapsed),

			ReadTime:  readTimeDelta,
			WriteTime: writeTimeDelta,

			ReadLatency:  &readLatency,
			WriteLatency: &writeLatency,
			ReadBusyPct:  &readBusy,
			WriteBusyPct: &writeBusy,

			InProgress: uint64(curr.QueueDepth),
		})
	}

	lastDiskPerf = currentPerf
	lastDiskTime = now
	return result, nil
}

func getDrivePerformance(driveIndex uint32) (winapi.DiskPerformance, error) {
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
		return winapi.DiskPerformance{}, fmt.Errorf("CreateFile failed: %w", err)
	}
	defer windows.CloseHandle(handle)

	var perf winapi.DiskPerformance
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		handle,
		winapi.IoctlDiskPerformance,
		nil,
		0,
		(*byte)(unsafe.Pointer(&perf)),
		uint32(unsafe.Sizeof(perf)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return winapi.DiskPerformance{}, fmt.Errorf("DeviceIoControl failed: %w", err)
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
