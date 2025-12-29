//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/metrics"
)

var (
	ntdll                        = syscall.NewLazyDLL("ntdll.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")
	procGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")
)

const (
	systemProcessorPerformanceInformation = 8
)

// Raw Windows structure for per-core CPU times
type systemProcessorPerformanceInfo struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
	_              uint32 // Padding
}

type loadAverages struct {
	sync.RWMutex
	lastUpdate time.Time
	Load1      float64
	Load5      float64
	Load15     float64
}

// Update calculates the new load average based on the current CPU usage and time elapsed.
func (la *loadAverages) Update(cpuPercent float64) {
	numCPU := float64(getProcessorCount())
	currentLoad := (cpuPercent / 100.0) * numCPU

	la.Lock()
	defer la.Unlock()

	now := time.Now()

	// First run
	if la.lastUpdate.IsZero() {
		la.Load1 = currentLoad
		la.Load5 = currentLoad
		la.Load15 = currentLoad
		la.lastUpdate = now
		return
	}

	timeDelta := now.Sub(la.lastUpdate).Seconds()
	la.lastUpdate = now

	decay1 := math.Exp(-timeDelta / 60.0)
	decay5 := math.Exp(-timeDelta / 300.0)
	decay15 := math.Exp(-timeDelta / 900.0)

	la.Load1 = la.Load1*decay1 + currentLoad*(1-decay1)
	la.Load5 = la.Load5*decay5 + currentLoad*(1-decay5)
	la.Load15 = la.Load15*decay15 + currentLoad*(1-decay15)
}

var (
	// Package-level state for delta calculation
	lastCPUTimes []systemProcessorPerformanceInfo
	loadAvg      *loadAverages
	loadAvgOnce  sync.Once
)

func getLoadAverages() *loadAverages {
	loadAvgOnce.Do(func() {
		loadAvg = &loadAverages{}
	})
	return loadAvg
}

// CollectCPU collects CPU metrics for Windows.
func CollectCPU(ctx context.Context) ([]metrics.Metric, error) {
	currentTimes, err := getSystemProcessPerformanceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to query CPU info: %w", err)
	}

	// First Run
	if len(lastCPUTimes) == 0 || len(lastCPUTimes) != len(currentTimes) {
		lastCPUTimes = currentTimes
		return nil, nil
	}

	overallPct, corePcts := calculateDeltas(currentTimes, lastCPUTimes)

	// Update baseline
	lastCPUTimes = currentTimes

	la := getLoadAverages()
	la.Update(overallPct)
	load1, load5, load15 := GetLoadAverages()

	metric := metrics.CPUMetric{
		Usage:     overallPct,
		CoreUsage: corePcts,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
	}

	return []metrics.Metric{metric}, nil
}

func getSystemProcessPerformanceInfo() ([]systemProcessorPerformanceInfo, error) {
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

func calculateDeltas(current, last []systemProcessorPerformanceInfo) (float64, []float64) {
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

type systemInfo struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

func GetLoadAverages() (load1, load5, load15 float64) {
	la := getLoadAverages()
	la.RLock()
	defer la.RUnlock()
	return la.Load1, la.Load5, la.Load15
}
