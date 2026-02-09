//go:build freebsd

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

// Package-level state for delta calculation.
var lastCPURawData map[string]CPURaw

// Standard C long is 8 bytes on amd64/arm64
const longSize = 8

// structSize represents the 40-byte size of kern.cp_times data
const structSize = 40

// CPUTime represents the C-style array of 5 longs returned by FreeBSD kernels.
type CPUTime struct {
	User uint64
	Nice uint64
	Sys  uint64
	Intr uint64
	Idle uint64
}

// loadAvgRaw represents the struct loadavg { uint32_t ldavg[3]; long fscale; }
// including the 4-byte padding for alignment.
type loadAvgRaw struct {
	Load   [3]uint32
	_      [4]byte // Padding
	Fscale uint64
}

func CollectCPU(ctx context.Context) ([]protocol.Metric, error) {
	cur, err := getCPUTimes()
	if err != nil {
		return nil, fmt.Errorf("getting cpu times: %w", err)
	}

	// Baseline
	if len(lastCPURawData) == 0 {
		lastCPURawData = cur
		return nil, nil
	}

	deltaMap, ok := calculateCPUDeltas(cur, lastCPURawData)
	if !ok {
		lastCPURawData = nil // reset on rollover or change
		return nil, nil
	}
	lastCPURawData = cur

	usage := percent(deltaMap["cpu"].Used, deltaMap["cpu"].Total)
	coreUsage := calcCoreUsage(deltaMap)

	load1, load5, load15, err := getLoadAvg()
	if err != nil {
		return nil, fmt.Errorf("getting load avg: %w", err)
	}

	return []protocol.Metric{protocol.CPUMetric{
		Usage:     usage,
		CoreUsage: coreUsage,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
	}}, nil
}

// calculateCPUDeltas takes the current and previous raw maps and returns a map containing
// the delta for each key (cpu, cpu0, ...)
// The FreeBSD build does not track IOWait/SoftIRQ/Steal/Guest/GuestNice
func calculateCPUDeltas(current, previous map[string]CPURaw) (map[string]CPUDelta, bool) {
	deltaMap := make(map[string]CPUDelta)

	for key, cur := range current {
		prev, ok := previous[key]
		if !ok {
			return nil, false
		}

		if cur.User < prev.User || cur.Nice < prev.Nice || cur.System < prev.System || cur.Idle < prev.Idle {
			return nil, false
		}

		delta := CPUDelta{}

		delta.User = cur.User - prev.User
		delta.Nice = cur.Nice - prev.Nice
		delta.System = cur.System - prev.System
		delta.Idle = cur.Idle - prev.Idle
		delta.IRQ = cur.IRQ - prev.IRQ
		// The remainder are not natively tracked in FreeBSD kern.cp_time
		delta.IOWait = 0
		delta.SoftIRQ = 0
		delta.Steal = 0
		delta.Guest = 0
		delta.GuestNice = 0

		delta.Total = delta.User + delta.Nice + delta.System + delta.Idle + delta.IRQ
		delta.Used = delta.Total - delta.Idle

		deltaMap[key] = delta
	}

	return deltaMap, true
}

func getCPUTimes() (map[string]CPURaw, error) {
	result := make(map[string]CPURaw)

	// Aggregate CPU times: kern.cp_time
	rawAgg, err := unix.SysctlRaw("kern.cp_time")
	if err != nil {
		return nil, err
	}

	aggStats, err := parseCPUTimes(rawAgg)
	if err != nil {
		return nil, fmt.Errorf("parsing aggregate: %w", err)
	}

	if len(aggStats) > 0 {
		agg := aggStats[0]
		result["cpu"] = CPURaw{
			User:   agg.User,
			Nice:   agg.Nice,
			System: agg.Sys,
			IRQ:    agg.Intr,
			Idle:   agg.Idle,
		}
	}

	// Per-core CPU times: kern.cp_times
	rawAll, err := unix.SysctlRaw("kern.cp_times")
	if err != nil {
		return nil, err
	}

	coreStats, err := parseCPUTimes(rawAll)
	if err != nil {
		return nil, fmt.Errorf("parsing per-core: %w", err)
	}

	for i, stats := range coreStats {
		key := fmt.Sprintf("cpu%d", i)
		result[key] = CPURaw{
			User:   stats.User,
			Nice:   stats.Nice,
			System: stats.Sys,
			IRQ:    stats.Intr,
			Idle:   stats.Idle,
		}
	}

	return result, nil
}

func getLoadAvg() (load1, load5, load15 float64, err error) {
	// vm.loadavg
	// returns: struct loadavg { fixpt_t ldavg[3]; long fscale; };
	// [4] [4] [4] [4 padding] [8]
	data, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	return parseLoadAvg(data)
}

// parseCPUTimes parses the raw beytes slice from kern.cp_time or kern.cp_times
// and returns a slice of CPUTime structs.
func parseCPUTimes(data []byte) ([]CPUTime, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	if len(data)%structSize != 0 {
		return nil, fmt.Errorf("data length %d is not a multiple of struct size %d", len(data), structSize)
	}

	count := len(data) / structSize
	results := make([]CPUTime, count)

	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// parseLoadAvg parses the raw byte slice from vm.loadavg
// It expects the C struct layout: { uint32[3], padding[4], uint64 }.
func parseLoadAvg(data []byte) (load1, load5, load15 float64, err error) {
	var raw loadAvgRaw

	if len(data) < (3*4 + 4 + 8) {
		return 0, 0, 0, fmt.Errorf("vm.loadavg data too short")
	}

	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &raw); err != nil {
		return 0, 0, 0, err
	}

	if raw.Fscale == 0 {
		return 0, 0, 0, fmt.Errorf("fscale is zero")
	}

	fltScale := float64(raw.Fscale)
	load1 = float64(raw.Load[0]) / fltScale
	load5 = float64(raw.Load[1]) / fltScale
	load15 = float64(raw.Load[2]) / fltScale

	return load1, load5, load15, nil
}

func calcCoreUsage(deltaMap map[string]CPUDelta) []float64 {
	numCores := len(deltaMap) - 1
	if numCores < 0 {
		return []float64{}
	}

	usage := make([]float64, numCores)

	for i := range numCores {
		coreKey := fmt.Sprintf("cpu%d", i)
		if delta, ok := deltaMap[coreKey]; ok && delta.Total > 0 {
			usage[i] = percent(delta.Used, delta.Total)
		}
	}

	return usage
}
