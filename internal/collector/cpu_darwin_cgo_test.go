//go:build darwin && cgo

package collector

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectCPU_FirstSampleNil(t *testing.T) {
	lastCPURawData = nil

	ctx := context.Background()
	metrics, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics != nil {
		t.Errorf("first sample should return nil, got %v", metrics)
	}
	if lastCPURawData == nil {
		t.Fatal("lastCPURawData should be populated after first sample")
	}
}

func TestCollectCPU_SecondSampleReturnsMetrics(t *testing.T) {
	lastCPURawData = nil

	ctx := context.Background()

	_, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("first sample: unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	metrics, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("second sample: unexpected error: %v", err)
	}
	if metrics == nil {
		t.Fatal("second sample returned nil")
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	cpu, ok := metrics[0].(protocol.CPUMetric)
	if !ok {
		t.Fatalf("expected CPUMetric, got %T", metrics[0])
	}

	t.Logf("Usage: %.2f%%", cpu.Usage)
	t.Logf("Cores: %d", len(cpu.CoreUsage))
	t.Logf("LoadAvg: %.2f %.2f %.2f", cpu.LoadAvg1, cpu.LoadAvg5, cpu.LoadAvg15)

	if cpu.Usage < 0 || cpu.Usage > 100 {
		t.Errorf("usage = %.2f, want 0-100", cpu.Usage)
	}
	if len(cpu.CoreUsage) == 0 {
		t.Error("CoreUsage is empty")
	}

	for i, core := range cpu.CoreUsage {
		if core < 0 || core > 100 {
			t.Errorf("core %d usage = %.2f, want 0-100", i, core)
		}
		t.Logf("core %d Usage: %f", i, core)
	}

	if cpu.LoadAvg1 < 0 || cpu.LoadAvg5 < 0 || cpu.LoadAvg15 < 0 {
		t.Errorf("negative load averages: %.2f %.2f %.2f", cpu.LoadAvg1, cpu.LoadAvg5, cpu.LoadAvg15)
	}
	if cpu.IOWait != 0 {
		t.Errorf("IOWait = %.2f, want 0 on darwin", cpu.IOWait)
	}
}

func TestCollectCPU_DeltasMonotonic(t *testing.T) {
	lastCPURawData = nil

	ctx := context.Background()

	CollectCPU(ctx)
	time.Sleep(200 * time.Millisecond)

	metrics1, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	metrics2, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics1 == nil || metrics2 == nil {
		t.Fatal("expected non-nil metrics for both samples")
	}

	cpu1 := metrics1[0].(protocol.CPUMetric)
	cpu2 := metrics2[0].(protocol.CPUMetric)

	if cpu1.Usage < 0 || cpu1.Usage > 100 {
		t.Errorf("sample 1 usage = %.2f, want 0-100", cpu1.Usage)
	}
	if cpu2.Usage < 0 || cpu2.Usage > 100 {
		t.Errorf("sample 2 usage = %.2f, want 0-100", cpu2.Usage)
	}
	if len(cpu1.CoreUsage) != len(cpu2.CoreUsage) {
		t.Errorf("core count mismatch: %d vs %d", len(cpu1.CoreUsage), len(cpu2.CoreUsage))
	}

	t.Logf("sample 1: usage=%.2f%%, cores=%d", cpu1.Usage, len(cpu1.CoreUsage))
	t.Logf("sample 2: usage=%.2f%%, cores=%d", cpu2.Usage, len(cpu2.CoreUsage))
}

func TestCollectCPU_Cancellation(t *testing.T) {
	lastCPURawData = nil

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Shouldn't panic
	_, _ = CollectCPU(ctx)
}

func TestReadCPURaw(t *testing.T) {
	raw, err := readCPURaw()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	numCPU := runtime.NumCPU()

	if len(raw) != numCPU+1 {
		t.Errorf("got %d entries, want %d (aggregate + %d cores)", len(raw), numCPU+1, numCPU)
	}

	agg, ok := raw["cpu"]
	if !ok {
		t.Fatal("missing aggregate cpu key")
	}
	if agg.Idle == 0 {
		t.Error("aggregate idle ticks = 0")
	}

	for i := range numCPU {
		key := fmt.Sprintf("cpu%d", i)
		core, ok := raw[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if core.Idle == 0 {
			t.Errorf("%s idle ticks = 0", key)
		}
	}

	var sumUser, sumSystem, sumIdle, sumNice uint64
	for i := range numCPU {
		core := raw[fmt.Sprintf("cpu%d", i)]
		sumUser += core.User
		sumSystem += core.System
		sumIdle += core.Idle
		sumNice += core.Nice
	}

	if agg.User != sumUser {
		t.Errorf("aggregate User=%d, sum of cores=%d", agg.User, sumUser)
	}
	if agg.System != sumSystem {
		t.Errorf("aggregate System=%d, sum of cores=%d", agg.System, sumSystem)
	}
	if agg.Idle != sumIdle {
		t.Errorf("aggregate Idle=%d, sum of cores=%d", agg.Idle, sumIdle)
	}
	if agg.Nice != sumNice {
		t.Errorf("aggregate Nice=%d, sum of cores=%d", agg.Nice, sumNice)
	}
}
