//go:build linux
// +build linux

package collector

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
)

const bytesPerSector float64 = 512.0

var (
	// Persistent state: Mape of device name to its cumulative I/O stats from the last run.
	lastDiskIORaw map[string]DiskIORaw

	// Package-level variable to track the last successful run time
	lastDiskIOTime time.Time
)

type DiskIORaw struct {
	DeviceName   string // Field 2
	ReadSectors  uint64 // Field 4 (512-byte sectors)
	WriteSectors uint64 // Field 8 (512-byte sectors)
	ReadTime     uint64 // Field 5 (ms)
	WriteTime    uint64 // Field 9 (ms)
	ReadOps      uint64 // Field 3 (total reads completed)
	WriteOps     uint64 // Field 7 (total writes completed)
	InProgress   uint64 // Field 11
}

type DiskIODelta struct {
	ReadOps      uint64
	ReadSectors  uint64
	WriteOps     uint64
	WriteSectors uint64
	ReadTime     uint64
	WriteTime    uint64
}

func MakeDiskIOCollector(cache *MountMap) CollectFunc {
	return func(ctx context.Context) ([]metrics.Metric, error) {
		return CollectDiskIO(ctx, cache)
	}
}

func CollectDiskIO(ctx context.Context, cache *MountMap) ([]metrics.Metric, error) {
	mountMap := loadMountMap(cache)

	currentRaw, err := parseProcDiskstats(mountMap)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if len(lastDiskIORaw) == 0 {
		lastDiskIORaw = currentRaw
		lastDiskIOTime = now
		return nil, nil
	}

	elapsed := now.Sub(lastDiskIOTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	result := make([]metrics.Metric, 0, len(currentRaw))

	for device, curr := range currentRaw {
		prev, ok := lastDiskIORaw[device]
		if !ok {
			continue
		}

		result = append(result, buildDiskIOMetric(device, curr, prev, elapsed))
	}

	lastDiskIORaw = currentRaw
	lastDiskIOTime = now

	return result, nil
}

func buildDiskIOMetric(device string, curr, prev DiskIORaw, elapsed float64) metrics.DiskIOMetric {
	readBytesDelta := float64(curr.ReadSectors-prev.ReadSectors) * bytesPerSector
	writeBytesDelta := float64(curr.WriteSectors-prev.WriteSectors) * bytesPerSector

	return metrics.DiskIOMetric{
		Device:     device,
		ReadBytes:  uint64(readBytesDelta / elapsed),
		WriteBytes: uint64(writeBytesDelta / elapsed),
		ReadOps:    rate(curr.ReadOps-prev.ReadOps, elapsed),
		WriteOps:   rate(curr.WriteOps-prev.WriteOps, elapsed),
		ReadTime:   curr.ReadTime - prev.ReadTime,
		WriteTime:  curr.WriteTime - prev.WriteTime,
		InProgress: curr.InProgress,
	}
}

func rate(delta uint64, seconds float64) uint64 {
	return uint64(float64(delta) / seconds)
}

func parseProcDiskstats(mountMap map[string]MountInfo) (map[string]DiskIORaw, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]DiskIORaw)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 12 {
			continue
		}

		device := fields[2]
		if _, monitored := mountMap[device]; !monitored {
			continue
		}

		result[device] = parseDiskIORaw(device, fields)
	}

	return result, scanner.Err()
}

func parseDiskIORaw(device string, fields []string) DiskIORaw {
	parse := func(index int) uint64 {
		v, _ := strconv.ParseUint(fields[index], 10, 64)
		return v
	}

	return DiskIORaw{
		DeviceName:   device,
		ReadOps:      parse(3),
		ReadSectors:  parse(5),
		ReadTime:     parse(6),
		WriteOps:     parse(7),
		WriteSectors: parse(9),
		WriteTime:    parse(10),
		InProgress:   parse(11),
	}
}
