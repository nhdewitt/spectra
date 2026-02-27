//go:build darwin

package collector

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectProcessRaw_Integration(t *testing.T) {
	ctx := context.Background()
	procs, totalMem, err := collectProcessRaw(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(procs) < 10 {
		t.Errorf("got %d procs, expected at least 10 on a running system", len(procs))
	}
	if totalMem <= 0 {
		t.Errorf("totalMem = %d, expected > 0", totalMem)
	}

	// Verify launchd is in the list
	var foundLaunchd bool
	for _, p := range procs {
		if p.PID == 1 {
			foundLaunchd = true
			if p.RSSBytes == 0 {
				t.Error("launchd RSS is 0")
			}
			t.Logf("launchd: RSS=%d CPU=%.1f%% State=%s", p.RSSBytes, p.CPUPercent, p.State)
			break
		}
	}

	if !foundLaunchd {
		t.Error("launchd (PID 1) not found in process list")
	}

	t.Logf("total processes: %d, totalMem: %d MB", len(procs), totalMem/1024/1024)
}

func TestCollectProcesses_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	plm, ok := metrics[0].(protocol.ProcessListMetric)
	if !ok {
		t.Fatalf("expected ProcessListMetric, got %T", metrics[0])
	}
	if len(plm.Processes) < 10 {
		t.Errorf("got %d processes, expected at least 10", len(plm.Processes))
	}

	t.Logf("collected %d processes", len(plm.Processes))
}

func BenchmarkCollectProcessRaw(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		collectProcessRaw(ctx)
	}
}
