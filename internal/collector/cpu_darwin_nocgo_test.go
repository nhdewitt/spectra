//go:build darwin && !cgo

package collector

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseCPUUsageLine_Basic(t *testing.T) {
	tests := []struct {
		line              string
		wantUser, wantSys float64
	}{
		{"5.26% user, 10.52% sys, 84.21% idle", 5.26, 10.52},
		{"0.0% user, 0.0% sys, 100.0% idle", 0.0, 0.0},
		{"50.00% user, 25.00% sys, 25.00% idle", 50.00, 25.00},
	}

	for _, tt := range tests {
		usage, err := parseCPUUsageLine(tt.line)
		if err != nil {
			t.Errorf("parseCPUUsageLine(%q): %v", tt.line, err)
			continue
		}
		want := tt.wantUser + tt.wantSys
		if usage < want-0.01 || usage > want+0.01 {
			t.Errorf("parseCPUUsageLine(%q) = %.2f, want %.2f", tt.line, usage, want)
		}
	}
}

func TestParseCPUUsageLine_MalformedFields(t *testing.T) {
	usage, err := parseCPUUsageLine("garbage")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if usage != 0 {
		t.Errorf("expected 0, got %.2f", usage)
	}
}

func TestParseCPUFromTop_TwoSamples(t *testing.T) {
	out := []byte(`Processes: 300 total, 2 running, 298 sleeping, 1200 threads
CPU usage: 10.00% user, 5.00% sys, 85.00% idle
SharedLibs: 200M resident
Processes: 300 total, 3 running, 297 sleeping, 1201 threads
CPU usage: 3.50% user, 2.10% sys, 94.40% idle
SharedLibs: 200M resident
`)

	usage, err := parseCPUFromTop(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use the second sample
	want := 5.60
	if usage < want-0.01 || usage > want+0.01 {
		t.Errorf("usage = %.2f, want %.2f", usage, want)
	}
}

func TestParseCPUFromTop_NoCPULine(t *testing.T) {
	out := []byte("Processes: 300 total\nSharedLibs: 200M resident\n")

	usage, err := parseCPUFromTop(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage != 0 {
		t.Errorf("expected 0, got %.2f", usage)
	}
}

func TestParseCPUFromTop_Empty(t *testing.T) {
	usage, err := parseCPUFromTop(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if usage != 0 {
		t.Errorf("expected 0, got %.2f", usage)
	}
}

func TestCollectCPU_NoCgo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	metrics, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	cpu, ok := metrics[0].(protocol.CPUMetric)
	if !ok {
		t.Fatalf("expected CPUMetric, got %T", metrics[0])
	}

	t.Logf("Usage: %.2f%%", cpu.Usage)
	t.Logf("LoadAvg: %.2f %.2f %.2f", cpu.LoadAvg1, cpu.LoadAvg5, cpu.LoadAvg15)

	if cpu.Usage < 0 || cpu.Usage > 100 {
		t.Errorf("usage = %.2f, want 0-100", cpu.Usage)
	}

	if len(cpu.CoreUsage) != 0 {
		t.Errorf("expected empty CoreUsage without cgo, got %d cores", len(cpu.CoreUsage))
	}
	if cpu.LoadAvg1 < 0 || cpu.LoadAvg5 < 0 || cpu.LoadAvg15 < 0 {
		t.Errorf("negative load averages: %.2f %.2f %.2f", cpu.LoadAvg1, cpu.LoadAvg5, cpu.LoadAvg15)
	}
}
