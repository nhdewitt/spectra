//go:build windows

package collector

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

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

func approxEqual_Windows(a, b, epsilon float64) bool {
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

	if !approxEqual_Windows(overall, expectedOverallUsage, 0.001) {
		t.Errorf("Overall Usage mismatch. Got: %.2f%%, Want: %.2f%%", overall, expectedOverallUsage)
	}

	if len(perCore) != numCores {
		t.Fatalf("Per-core count mismatch. Got: %d, Want: %d", len(perCore), numCores)
	}

	for i := range perCore {
		if !approxEqual_Windows(perCore[i], expectedCoreUsage[i], 0.001) {
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

func TestEMA(t *testing.T) {
	tests := []struct {
		name     string
		prev     float64
		current  float64
		interval float64
		period   float64
		want     float64
		epsilon  float64
	}{
		{
			name:     "no time elapsed",
			prev:     5.0,
			current:  10.0,
			interval: 0,
			period:   60,
			want:     5.0, // prev * 1 + current * 0
			epsilon:  0.001,
		},
		{
			name:     "one period elapsed",
			prev:     0.0,
			current:  10.0,
			interval: 60,
			period:   60,
			want:     10.0 * (1 - math.Exp(-1)),
			epsilon:  0.01,
		},
		{
			name:     "very long interval",
			prev:     100.0,
			current:  5.0,
			interval: 6000,
			period:   60,
			want:     5.0,
			epsilon:  0.01,
		},
		{
			name:     "steady state",
			prev:     50.0,
			current:  50.0,
			interval: 30,
			period:   60,
			want:     50.0,
			epsilon:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ema(tt.prev, tt.current, tt.interval, tt.period)
			if !approxEqual_Windows(got, tt.want, tt.epsilon) {
				t.Errorf("ema() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestLoadAverages_FirstUpdate(t *testing.T) {
	la := &loadAverages{}

	load1, load5, load15 := la.Update(50.0)

	if load1 != load5 || load5 != load15 {
		t.Errorf("first update should set equal values: got %f, %f, %f", load1, load5, load15)
	}
	if load1 <= 0 {
		t.Error("load should be positive after update")
	}
}

func TestCalculateCPUDeltas_Overflow(t *testing.T) {
	t0 := []systemProcessorPerformanceInfo{
		{UserTime: math.MaxInt64 - 100, KernelTime: math.MaxInt64 - 100, IdleTime: math.MaxInt64 - 200},
	}
	t1 := []systemProcessorPerformanceInfo{
		{UserTime: 100, KernelTime: 100, IdleTime: 50}, // wrapped
	}

	overall, perCore := calculateCPUDeltas(t1, t0)

	if overall != 0 {
		t.Errorf("expected 0%% on overflow, got %f", overall)
	}

	for i, usage := range perCore {
		if usage != 0 {
			t.Errorf("core %d: expected 0%% on overflow, got %f", i, usage)
		}
	}
}

func TestCalculateCPUDeltas_ZeroDelta(t *testing.T) {
	times := makeMockTimes(100, 50, 200, 2)

	overall, perCore := calculateCPUDeltas(times, times)

	if overall != 0 {
		t.Errorf("expected 0%% usage with no delta, got %f", overall)
	}

	for i, usage := range perCore {
		if usage != 0 {
			t.Errorf("core %d: expected 0%% usage, got %f", i, usage)
		}
	}
}

func TestCalculateCPUDeltas_SingleCore(t *testing.T) {
	t0 := makeMockTimes(0, 0, 100, 1)
	t1 := makeMockTimes(0, 0, 100, 1)

	t1[0].UserTime += 75
	t1[0].IdleTime += 25
	t1[0].KernelTime += 25

	overall, perCore := calculateCPUDeltas(t1, t0)

	if len(perCore) != 1 {
		t.Fatalf("expected 1 core, got %d", len(perCore))
	}
	if !approxEqual_Windows(overall, 75.0, 0.001) {
		t.Errorf("overall = %f, want 75.0", overall)
	}
	if !approxEqual_Windows(perCore[0], 75.0, 0.001) {
		t.Errorf("core 0 = %f, want 75.0", perCore[0])
	}
}

func TestLoadAverages_Convergence(t *testing.T) {
	la := &loadAverages{}

	// Simulate sustained 100% CPU load
	for range 100 {
		la.Update(100.0)
		la.lastUpdate = la.lastUpdate.Add(-5 * time.Second)
	}

	load1, load5, load15 := la.load1, la.load5, la.load15

	if load1 < load5 || load5 < load15 {
		t.Logf("convergence order: load1=%f, load5=%f, load15=%f", load1, load5, load15)
	}
}

func BenchmarkEMA(b *testing.B) {
	for b.Loop() {
		_ = ema(5.0, 10.0, 5.0, 60.0)
	}
}

func BenchmarkLoadAverages_Update(b *testing.B) {
	la := &loadAverages{}

	b.ResetTimer()
	for b.Loop() {
		la.Update(50.0)
	}
}
