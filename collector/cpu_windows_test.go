//go:build windows
// +build windows

package collector

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
	"github.com/shirou/gopsutil/v3/cpu"
)

const epsilon = 0.001

// Helper function to create a clean TimesStat slice for testing
func makeCPUTimesStat(user, system, idle float64, count int) []cpu.TimesStat {
	stats := make([]cpu.TimesStat, count+1)

	// Aggregate CPU (idx 0)
	stats[0] = cpu.TimesStat{
		User:      user * float64(count),
		System:    system * float64(count),
		Idle:      idle * float64(count),
		Nice:      0.0,
		Iowait:    0.0,
		Irq:       0.0,
		Softirq:   0.0,
		Steal:     0.0,
		Guest:     0.0,
		GuestNice: 0.0,
	}

	// Individual Cores (idx 1-count)
	for i := 1; i <= count; i++ {
		stats[i] = cpu.TimesStat{
			CPU:       fmt.Sprintf("cpu%d", i-1),
			User:      user,
			System:    system,
			Idle:      idle,
			Nice:      0.0,
			Iowait:    0.0,
			Irq:       0.0,
			Softirq:   0.0,
			Steal:     0.0,
			Guest:     0.0,
			GuestNice: 0.0,
		}
	}
	return stats
}

// Helper to check float percentage equality
func approxEqual_Windows(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestCalculateDeltaWindows_Logic(t *testing.T) {
	// 4-core system
	numCores := 4

	// Sample 1: Baseline
	// all cores idle for 100 seconds total
	t0 := makeCPUTimesStat(0, 0, 100, numCores)

	// Sample 2: 1 sec interval
	// Core 0: 50% used
	t1_c0 := cpu.TimesStat{User: 0.5, Idle: 0.5}
	// Core 1: 100% used
	t1_c1 := cpu.TimesStat{User: 1.0, Idle: 0.0}
	// Core 2: 0% used
	t1_c2 := cpu.TimesStat{User: 0.0, Idle: 1.0}
	// Core 3: 75% used
	t1_c3 := cpu.TimesStat{User: 0.5, System: 0.25, Idle: 0.25}

	t1 := make([]cpu.TimesStat, numCores+1)
	t1[1] = t0[1]
	t1[2] = t0[2]
	t1[3] = t0[3]
	t1[4] = t0[4]

	// Apply deltas to T1 (simulates second call)
	t1[1].User += t1_c0.User
	t1[1].Idle += t1_c0.Idle
	t1[2].User += t1_c1.User
	t1[2].Idle += t1_c1.Idle
	t1[3].User += t1_c2.User
	t1[3].Idle += t1_c2.Idle
	t1[4].User += t1_c3.User
	t1[4].System += t1_c3.System
	t1[4].Idle += t1_c3.Idle

	// Recalculate aggregate in T1
	t1_agg_user := t1[1].User + t1[2].User + t1[3].User + t1[4].User
	t1_agg_system := t1[1].System + t1[2].System + t1[3].System + t1[4].System
	t1_agg_idle := t1[1].Idle + t1[2].Idle + t1[3].Idle + t1[4].Idle
	t1[0].User = t1_agg_user
	t1[0].System = t1_agg_system
	t1[0].Idle = t1_agg_idle

	overall, perCore := calculateDeltaWindows(t1, t0)

	expectedCoreUsage := []float64{
		(0.5 / 1.0) * 100,  // Core 0: 50%
		(1.0 / 1.0) * 100,  // Core 1: 100%
		(0.0 / 1.0) * 100,  // Core 2: 0%
		(0.75 / 1.0) * 100, // Core 3: 75%
	}

	// Aggregate Expected:
	// Total aggregate time delta is 4 seconds
	// Total aggregate used time delta is (0.5 + 1.0 + 0.0 + 0.75) = 2.25 seconds
	// Overall Usage = (2.25 / 4.0) * 100 = 56.25%
	expectedOverallUsage := 56.25

	if !approxEqual_Windows(overall, expectedOverallUsage) {
		t.Errorf("Overall Usage mismatch. Got: %.2f%%, Want: %.2f%%", overall, expectedOverallUsage)
	}

	if len(perCore) != numCores {
		t.Fatalf("Per-core count mismatch. Got: %d, Want: %d", len(perCore), numCores)
	}

	for i := range perCore {
		if !approxEqual_Windows(perCore[i], expectedCoreUsage[i]) {
			t.Errorf("Core %d Usage mismatch. Got: %.2f%%, Want: %.2f%%", i, perCore[i], expectedCoreUsage[i])
		}
	}
}

func TestCollectCPUWindows_StateManagement(t *testing.T) {
	lastCPUTimes = nil
	ctx := context.Background()

	// First call (baseline)
	// Should return nil metrics and nil error
	metrics1, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if metrics1 != nil {
		t.Error("First call should return nil (baseline)")
	}
	if len(lastCPUTimes) == 0 {
		t.Fatal("lastCPUTimes should have been populated after first call")
	}

	// Second call
	// Simulate a time gap and core usage increase for a valid result
	time.Sleep(10 * time.Millisecond)

	metrics2, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if metrics2 == nil {
		t.Fatal("Second call expected metrics, got nil")
	}
	if len(metrics2) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics2))
	}

	cpuMetric, ok := metrics2[0].(metrics.CPUMetric)
	if !ok {
		t.Fatal("Returned metric is not CPUMetric type")
	}
	if cpuMetric.Usage < 0 || cpuMetric.Usage > 100 {
		t.Errorf("Usage percentage %.2f is out of expected range (0-100)", cpuMetric.Usage)
	}
}
