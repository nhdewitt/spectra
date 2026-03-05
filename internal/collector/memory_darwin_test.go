//go:build darwin

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestCollectMemory(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m := metrics[0].(protocol.MemoryMetric)

	t.Logf("Total:		%d MB", m.Total/1024/1024)
	t.Logf("Available:	%d MB", m.Available/1024/1024)
	t.Logf("Used:		%d MB (%.1f%%)", m.Used/1024/1024, m.UsedPct)
	t.Logf("Swap:		%d MB total, %d MB used (%.1f%%)", m.SwapTotal/1024/1024, m.SwapUsed/1024/1024, m.SwapPct)

	if m.Total == 0 {
		t.Error("mem Total is 0")
	}
	if m.Available == 0 {
		t.Error("mem Available is 0")
	}
	if m.Available > m.Total {
		t.Errorf("mem Available (%d) > mem Total (%d)", m.Available, m.Total)
	}
	if m.Used > m.Total {
		t.Errorf("mem Used (%d) > mem Total (%d)", m.Used, m.Total)
	}
	if m.UsedPct < 0 || m.UsedPct > 100 {
		t.Errorf("mem UsedPct = %.2f, want 0-100", m.UsedPct)
	}
	if m.SwapPct < 0 || m.SwapPct > 100 {
		t.Errorf("mem SwapPct = %.2f, want 0-100", m.SwapPct)
	}
}

func TestParseMemInfo(t *testing.T) {
	raw, err := parseMemInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total == 0 {
		t.Error("mem Total is 0")
	}
	if raw.Available == 0 {
		t.Error("mem Available is 0")
	}
	if raw.Available > raw.Total {
		t.Errorf("mem Available (%d) > mem Total (%d)", raw.Available, raw.Total)
	}

	t.Logf("Total: %d, Available: %d, SwapTotal: %d, SwapFree: %d", raw.Total, raw.Available, raw.SwapTotal, raw.SwapFree)
}

func TestParseSwapUsage(t *testing.T) {
	swap, err := parseSwapUsage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Swap: total=%d avail=%d used=%d", swap.Total, swap.Avail, swap.Used)

	// Used+Avail should roughly equal total
	if swap.Total > 0 {
		sum := swap.Avail + swap.Used
		if sum > swap.Total*2 {
			t.Errorf("swap avail+used (%d) is more than double total (%d)", sum, swap.Total)
		}
	}
}

func TestParseSwapUsage_Synthetic(t *testing.T) {
	swap := xswUsage{
		Total: 2147483648,
		Avail: 1073741824,
		Used:  1073741824,
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, swap); err != nil {
		t.Fatal(err)
	}

	var parsed xswUsage
	if err := binary.Read(bytes.NewReader(buf.Bytes()), binary.LittleEndian, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Total != swap.Total {
		t.Errorf("Total = %d, want %d", parsed.Total, swap.Total)
	}
	if parsed.Avail != swap.Avail {
		t.Errorf("Avail = %d, want %d", parsed.Avail, swap.Avail)
	}
	if parsed.Used != swap.Used {
		t.Errorf("Used = %d, want %d", parsed.Used, swap.Used)
	}
}

func TestSysctlInt(t *testing.T) {
	val, err := sysctlInt("hw.memsize")
	if err != nil {
		t.Fatal(err)
	}
	if val == 0 {
		t.Error("hw.memsize returned 0")
	}
	t.Logf("hw.memsize = %d bytes (%d GB)", val, val/1024/1024/1024)
}

func TestSysctlInt_PageSize(t *testing.T) {
	val, err := sysctlInt("hw.pagesize")
	if err != nil {
		t.Fatal(err)
	}

	// Intel: 4096, Silicon: 16384
	if val != 4096 && val != 16384 {
		t.Errorf("hw.pagesize = %d, expected 4096 or 16384", val)
	}
	t.Logf("hw.pagesize = %d", val)
}

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectMemory(ctx)
	}
}

func BenchmarkParseSwapUsage(b *testing.B) {
	for b.Loop() {
		_, _ = parseSwapUsage()
	}
}
