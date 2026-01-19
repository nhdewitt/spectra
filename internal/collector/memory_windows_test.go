//go:build windows

package collector

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectMemory_Integration(t *testing.T) {
	data, err := CollectMemory(context.Background())
	if err != nil {
		t.Fatalf("CollectMemory failed: %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("Expected 1 memory metric, got %d", len(data))
	}

	m, ok := data[0].(protocol.MemoryMetric)
	if !ok {
		t.Fatalf("Expected MemoryMetric, got %T", data[0])
	}

	t.Logf("Total RAM: %d bytes (%.2f GB)", m.Total, float64(m.Total)/1024/1024/1024)
	t.Logf("Used RAM:  %d bytes (%.2f%%)", m.Used, m.UsedPct)
	t.Logf("Swap/Page: %d bytes", m.SwapTotal)

	if m.Total == 0 {
		t.Error("Total memory reported as 0")
	}
	if m.Used > m.Total {
		t.Errorf("Used memory (%d) > Total memory (%d)", m.Used, m.Total)
	}
	if m.Available == 0 {
		t.Error("Available memory reported as 0")
	}
}

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectMemory(ctx)
	}
}
