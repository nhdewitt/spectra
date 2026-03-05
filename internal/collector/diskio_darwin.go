//go:build darwin

package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

var (
	lastDiskIORaw  map[string]DiskIORaw
	lastDiskIOTime time.Time
)

type DiskIORaw struct {
	DeviceName string
	ReadBytes  uint64
	WriteBytes uint64
	ReadTime   uint64 // ms
	WriteTime  uint64 // ms
	ReadOps    uint64
	WriteOps   uint64
}

// MakeDiskIOCollector creates a disk IO collector for Darwin.
// The cache parameter is accepted for interface consistency
// with Linux/FreeBSD but is unused; ioreg reports physical disk
// names which don't map to the partition-level device names in
// DriveCache.
func MakeDiskIOCollector(_ *DriveCache) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return CollectDiskIO(ctx)
	}
}

func CollectDiskIO(ctx context.Context) ([]protocol.Metric, error) {
	currentRaw, err := readDiskIOStats(ctx)
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

	result := make([]protocol.Metric, 0, len(currentRaw))

	for device, cur := range currentRaw {
		prev, ok := lastDiskIORaw[device]
		if !ok {
			continue
		}
		result = append(result, buildDiskIOMetric(device, cur, prev, elapsed))
	}

	lastDiskIORaw = currentRaw
	lastDiskIOTime = now

	return result, nil
}

func buildDiskIOMetric(device string, cur, prev DiskIORaw, elapsed float64) protocol.DiskIOMetric {
	return protocol.DiskIOMetric{
		Device:     device,
		ReadBytes:  uint64(float64(delta(cur.ReadBytes, prev.ReadBytes)) / elapsed),
		WriteBytes: uint64(float64(delta(cur.WriteBytes, prev.WriteBytes)) / elapsed),
		ReadOps:    rate(cur.ReadOps-prev.ReadOps, elapsed),
		WriteOps:   rate(cur.WriteOps-prev.WriteOps, elapsed),
		ReadTime:   delta(cur.ReadTime, prev.ReadTime),
		WriteTime:  delta(cur.WriteTime, prev.WriteTime),
	}
}

// readDiskIOStats shells out to ioreg to get per-disk cumulative IO stats.
//
// Output structure:
//
// +-o IOBlockStorageDriver  <class IOBlockStorageDriver, ...>
//
//	| {
//	|   ...
//	|   "Statistics" = {"Operations (Write)"=1012953,"Latency Time (Write)"=0,"Bytes (Read)"=31897657344,"Errors (Write)"=0,"Total Time (Read)"=658433335156,"Latency Time (Read)"=0,"Retries (Read)"=0,"Errors (Read)"=0,"Total Time (Write)"=67313671632,"Bytes (Write)"=14319661056,"Operations (Read)"=2416853,"Retries (Write)"=0}
//	|   ...
//	| }
//	|
//	+-o APPLE SSD AP0128Q Media  <class IOMedia, ...>
func readDiskIOStats(ctx context.Context) (map[string]DiskIORaw, error) {
	out, err := exec.CommandContext(
		ctx, "ioreg", "-d", "2", "-c",
		"IOBlockStorageDriver", "-r", "-w", "0",
	).Output()
	if err != nil {
		return nil, err
	}

	return parseIoregOutput(out), nil
}

func parseIoregOutput(out []byte) map[string]DiskIORaw {
	result := make(map[string]DiskIORaw)
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var stats map[string]uint64

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, `"Statistics"`) && strings.Contains(trimmed, "{") {
			stats = parseStatsDict(trimmed)
			continue
		}

		if stats != nil && strings.Contains(trimmed, "class IOMedia") {
			name := parseIOMediaName(trimmed)
			if name != "" {
				result[name] = DiskIORaw{
					DeviceName: name,
					ReadBytes:  stats[`Bytes (Read)`],
					WriteBytes: stats[`Bytes (Write)`],
					ReadOps:    stats[`Operations (Read)`],
					WriteOps:   stats[`Operations (Write)`],
					ReadTime:   stats[`Total Time (Read)`] / 1_000_000,  // ns->ms
					WriteTime:  stats[`Total Time (Write)`] / 1_000_000, // ns->ms
				}
			}
			stats = nil
		}
	}

	return result
}

// parseStatsDict parses a single-line ioreg Statistics dictionary.
// It converts "=" from ioreg to ":", converts to JSON, and unmarshals.
func parseStatsDict(line string) map[string]uint64 {
	start := strings.Index(line, "{")
	end := strings.LastIndex(line, "}")
	if start < 0 || end <= start {
		return nil
	}

	inner := line[start : end+1]
	if idx := strings.Index(inner, "{"); idx >= 0 {
		if end2 := strings.LastIndex(inner, "}"); end2 > idx {
			inner = inner[idx : end2+1]
		}
	}

	jsonStr := strings.ReplaceAll(inner, "\"=", "\":")
	result := make(map[string]uint64)
	json.Unmarshal([]byte(jsonStr), &result)

	return result
}

func parseIOMediaName(line string) string {
	_, after, ok := strings.Cut(line, "+-o ")
	if !ok {
		return ""
	}

	rest := after
	bracket := strings.Index(rest, "<")
	if bracket <= 0 {
		return ""
	}

	name := strings.TrimSpace(rest[:bracket])
	name = strings.TrimSuffix(name, " Media")

	return name
}
