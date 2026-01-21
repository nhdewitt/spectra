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
	if m.Available > m.Total {
		t.Errorf("Available (%d) > Total (%d)", m.Available, m.Total)
	}

	sum := m.Used + m.Available
	if sum != m.Total {
		t.Logf("Note: Used + Available = %d, Total = %d (diff: %d)", sum, m.Total, m.Total-sum)
	}

	if m.UsedPct < 0 || m.UsedPct > 100 {
		t.Errorf("UsedPct out of range: %.2f", m.UsedPct)
	}
	if m.SwapPct < 0 || m.SwapPct > 100 {
		t.Errorf("SwapPct out of range: %.2f", m.SwapPct)
	}

	minRAM := uint64(512 * 1024 * 1024)
	if m.Total < minRAM {
		t.Errorf("Total RAM %d seems too low", m.Total)
	}
}

func TestCollectMemory_Consistency(t *testing.T) {
	ctx := context.Background()

	m1, err := CollectMemory(ctx)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	m2, err := CollectMemory(ctx)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	mem1 := m1[0].(protocol.MemoryMetric)
	mem2 := m2[0].(protocol.MemoryMetric)

	if mem1.Total != mem2.Total {
		t.Errorf("Total changed between calls: %d vs %d", mem1.Total, mem2.Total)
	}
	if mem1.SwapTotal != mem2.SwapTotal {
		t.Errorf("SwapTotal changed between calls: %d vs %d", mem1.SwapTotal, mem2.SwapTotal)
	}
}

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectMemory(ctx)
	}
}
