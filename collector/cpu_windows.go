//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"

	"github.com/nhdewitt/raspimon/metrics"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/v3/cpu"
)

// Package-level state for delta calculation
var lastCPUTimes []cpu.TimesStat

// CollectCPU collects CPU metrics for Windows.
func CollectCPU(ctx context.Context) ([]metrics.Metric, error) {
	// Get raw CPU times
	currentTimes, err := cpu.TimesWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU times: %w", err)
	}

	// First run - baseline check
	if len(lastCPUTimes) == 0 {
		lastCPUTimes = currentTimes
		return nil, nil
	}

	// Calculate usage deltas
	overallPercent, coreUsage := calculateDeltaWindows(currentTimes, lastCPUTimes)

	// Calculate load averages
	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		loadAvg = &load.AvgStat{}
	}

	// Update baseline
	lastCPUTimes = currentTimes

	// Populate struct
	metric := metrics.CPUMetric{
		Usage:     overallPercent,
		CoreUsage: coreUsage,
		LoadAvg1:  loadAvg.Load1,
		LoadAvg5:  0.0,
		LoadAvg15: 0.0,
	}

	return []metrics.Metric{metric}, nil
}

func calculateDeltaWindows(current, last []cpu.TimesStat) (overall float64, perCore []float64) {
	numCores := len(current) - 1
	perCore = make([]float64, 0, numCores)

	for i, cur := range current {
		if i >= len(last) {
			continue
		}
		delta := CPUDeltaWindows{}

		delta.User = cur.User - last[i].User
		delta.Nice = cur.Nice - last[i].Nice
		delta.System = cur.System - last[i].System
		delta.Idle = cur.Idle - last[i].Idle
		delta.IOWait = cur.Iowait - last[i].Iowait
		delta.IRQ = cur.Irq - last[i].Irq
		delta.SoftIRQ = cur.Softirq - last[i].Softirq
		delta.Steal = cur.Steal - last[i].Steal
		delta.Guest = cur.Guest - last[i].Guest
		delta.GuestNice = cur.GuestNice - last[i].GuestNice
		// Don't add Guest and GuestNice to total (see cpu.go)
		delta.Total = delta.User + delta.Nice + delta.System + delta.Idle + delta.IOWait + delta.IRQ + delta.SoftIRQ + delta.Steal
		delta.Used = delta.Total - (delta.Idle + delta.IOWait)

		if i == 0 {
			overall = percent(delta.Used, delta.Total)
		} else {
			perCore = append(perCore, percent(delta.Used, delta.Total))
		}
	}

	return overall, perCore
}
