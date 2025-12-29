//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/metrics"
)

type loadAverages struct {
	sync.RWMutex
	lastUpdate time.Time
	load1      float64
	load5      float64
	load15     float64
}

var (
	// Package-level state for delta calculation
	lastCPUTimes []systemProcessorPerformanceInfo
	loadAvg      *loadAverages
	loadAvgOnce  sync.Once
)

// Update calculates the new load average based on the current CPU usage and time elapsed.
func (la *loadAverages) Update(cpuPercent float64) (load1, load5, load15 float64) {
	currentLoad := (cpuPercent / 100.0) * float64(getProcessorCount())

	la.Lock()
	defer la.Unlock()

	now := time.Now()

	// First run
	if la.lastUpdate.IsZero() {
		la.load1 = currentLoad
		la.load5 = currentLoad
		la.load15 = currentLoad
		la.lastUpdate = now
		return la.load1, la.load5, la.load15
	}

	timeDelta := now.Sub(la.lastUpdate).Seconds()
	la.lastUpdate = now

	la.load1 = ema(la.load1, currentLoad, timeDelta, 60)
	la.load5 = ema(la.load5, currentLoad, timeDelta, 300)
	la.load15 = ema(la.load15, currentLoad, timeDelta, 900)

	return la.load1, la.load5, la.load15
}

func ema(prev, current, interval, period float64) float64 {
	decay := math.Exp(-interval / period)
	return prev*decay + current*(1-decay)
}

// GetLoadAverages returns current 1/5/15 minute load averages.
func GetLoadAverages() (load1, load5, load15 float64) {
	la := getLoadAverages()
	la.RLock()
	defer la.RUnlock()
	return la.load1, la.load5, la.load15
}

func getLoadAverages() *loadAverages {
	loadAvgOnce.Do(func() {
		loadAvg = &loadAverages{}
	})
	return loadAvg
}

// CollectCPU collects CPU metrics for Windows.
func CollectCPU(ctx context.Context) ([]metrics.Metric, error) {
	currentTimes, err := getSystemProcessorPerformanceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to query CPU info: %w", err)
	}

	// First Run
	if len(lastCPUTimes) == 0 || len(lastCPUTimes) != len(currentTimes) {
		lastCPUTimes = currentTimes
		return nil, nil
	}

	overallPct, corePcts := calculateCPUDeltas(currentTimes, lastCPUTimes)

	// Update baseline
	lastCPUTimes = currentTimes

	load1, load5, load15 := getLoadAverages().Update(overallPct)

	metric := metrics.CPUMetric{
		Usage:     overallPct,
		CoreUsage: corePcts,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
	}

	return []metrics.Metric{metric}, nil
}

func getSystemProcessorPerformanceInfo() ([]systemProcessorPerformanceInfo, error) {
	numCores := getProcessorCount()

	result := make([]systemProcessorPerformanceInfo, numCores)

	bufferSize := uintptr(len(result)) * unsafe.Sizeof(result[0])
	var returnLength uint32

	ret, _, _ := procNtQuerySystemInformation.Call(
		uintptr(systemProcessorPerformanceInformation),
		uintptr(unsafe.Pointer(&result[0])),
		bufferSize,
		uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret != 0 {
		return nil, fmt.Errorf("NtQuerySystemInformation failed with status 0x%x", ret)
	}

	return result, nil
}

func calculateCPUDeltas(current, last []systemProcessorPerformanceInfo) (float64, []float64) {
	var totalSystemUsed, totalSystemTime float64
	perCore := make([]float64, len(current))

	for i := range current {
		c := current[i]
		l := last[i]

		// Raw Deltas
		deltaIdle := float64(c.IdleTime - l.IdleTime)
		deltaKernel := float64(c.KernelTime - l.KernelTime)
		deltaUser := float64(c.UserTime - l.UserTime)

		// Total active time = (Kernel - Idle) + User
		// Total elapsed time = Kernel + User

		deltaTotal := deltaKernel + deltaUser
		deltaUsed := deltaTotal - deltaIdle

		if deltaTotal > 0 {
			corePct := (deltaUsed / deltaTotal) * 100.0
			perCore[i] = corePct

			totalSystemUsed += deltaUsed
			totalSystemTime += deltaTotal
		}
	}

	overall := 0.0
	if totalSystemTime > 0 {
		overall = (totalSystemUsed / totalSystemTime) * 100.0
	}

	return overall, perCore
}

func getProcessorCount() int {
	var info systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&info)))
	return int(info.NumberOfProcessors)
}
