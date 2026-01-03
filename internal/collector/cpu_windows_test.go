//go:build windows

package collector

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const epsilon = 0.001

// Helper to create native Windows structs for testing.
// IMPORTANT: Simulates Windows behavior where KernelTime INCLUDES IdleTime.
func makeMockTimes(user, system, idle int64, count int) []systemProcessorPerformanceInfo {
	stats := make([]systemProcessorPerformanceInfo, count)

	for i := 0; i < count; i++ {
		stats[i] = systemProcessorPerformanceInfo{
			UserTime: user,
			// The Windows Trap: KernelTime includes IdleTime
			KernelTime: system + idle,
			IdleTime:   idle,
		}
	}
	return stats
}

func approxEqual_Windows(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestCalculateDeltaWindows_Logic(t *testing.T) {
	// 4-core system
	numCores := 4

	// Sample 1: Baseline (T0)
	// All cores idle for 100 ticks
	// User=0, System=0, Idle=100 -> Kernel=100
	t0 := makeMockTimes(0, 0, 100, numCores)

	// Sample 2: Update (T1)
	// We start with a copy of T0 and add ticks to simulate usage
	t1 := makeMockTimes(0, 0, 100, numCores)

	// --- Core 0: 50% Used ---
	// 50 User, 50 Idle
	// Delta Total = 100
	t1[0].UserTime += 50
	t1[0].IdleTime += 50
	t1[0].KernelTime += 50 // (System 0 + Idle 50)

	// --- Core 1: 100% Used ---
	// 100 User, 0 Idle
	// Delta Total = 100
	t1[1].UserTime += 100
	t1[1].IdleTime += 0
	t1[1].KernelTime += 0 // (System 0 + Idle 0)

	// --- Core 2: 0% Used ---
	// 0 User, 100 Idle
	// Delta Total = 100
	t1[2].UserTime += 0
	t1[2].IdleTime += 100
	t1[2].KernelTime += 100 // (System 0 + Idle 100)

	// --- Core 3: 75% Used ---
	// 50 User, 25 System, 25 Idle
	// Delta Total = 100
	t1[3].UserTime += 50
	t1[3].IdleTime += 25
	t1[3].KernelTime += 50 // (System 25 + Idle 25)

	// Run the calculation
	overall, perCore := calculateCPUDeltas(t1, t0)

	expectedCoreUsage := []float64{
		50.0,  // Core 0
		100.0, // Core 1
		0.0,   // Core 2
		75.0,  // Core 3
	}

	// Aggregate Calculation:
	// Total Deltas across all cores = 400 ticks
	// Used Deltas:
	// C0: 50
	// C1: 100
	// C2: 0
	// C3: 75 (50 user + 25 system)
	// Total Used = 225
	// Overall = 225 / 400 = 56.25%
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
	// Reset package-level state
	lastCPUTimes = nil
	ctx := context.Background()

	// 1. First call (Baseline)
	// Should return nil because we need two data points to calc %
	metrics1, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if metrics1 != nil {
		t.Error("First call should return nil (baseline population only)")
	}
	if len(lastCPUTimes) == 0 {
		t.Fatal("lastCPUTimes should have been populated after first call")
	}

	// 2. Second call
	// Sleep briefly to ensure non-zero time deltas (prevents divide by zero)
	time.Sleep(50 * time.Millisecond)

	metrics2, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if metrics2 == nil {
		t.Fatal("Second call expected metrics, got nil")
	}
	if len(metrics2) != 1 {
		t.Fatalf("Expected 1 metric struct, got %d", len(metrics2))
	}

	// 3. Validation
	cpuMetric, ok := metrics2[0].(protocol.CPUMetric)
	if !ok {
		t.Fatal("Returned metric is not CPUMetric type")
	}

	// Sanity check range
	if cpuMetric.Usage < 0 || cpuMetric.Usage > 100 {
		t.Errorf("Usage percentage %.2f is out of expected range (0-100)", cpuMetric.Usage)
	}

	// Ensure we got core data (assuming the test runner machine has at least 1 core)
	if len(cpuMetric.CoreUsage) == 0 {
		t.Error("Expected per-core usage data, got empty slice")
	}
}
