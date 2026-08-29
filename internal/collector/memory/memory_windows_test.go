//go:build windows

package memory

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const mb = uint64(1024 * 1024)

func TestSumPageFiles_NoPageFiles(t *testing.T) {
	total, used := sumPageFiles(nil)
	if total != 0 || used != 0 {
		t.Errorf("sumPageFiles(nil) = (%d, %d), want (0, 0)", total, used)
	}

	total, used = sumPageFiles([]Win32_PageFileUsage{})
	if total != 0 || used != 0 {
		t.Errorf("sumPageFiles(empty) = (%d, %d), want (0, 0)", total, used)
	}
}

func TestSumPageFiles_SingleConvertsMBToBytes(t *testing.T) {
	rows := []Win32_PageFileUsage{
		{Name: `C:\pagefile.sys`, AllocatedBaseSize: 4096, CurrentUsage: 512},
	}

	total, used := sumPageFiles(rows)

	if want := 4096 * mb; total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
	if want := 512 * mb; used != want {
		t.Errorf("used = %d, want %d", used, want)
	}
}

func TestSumPageFiles_MultipleAreSummed(t *testing.T) {
	rows := []Win32_PageFileUsage{
		{Name: `C:\pagefile.sys`, AllocatedBaseSize: 4096, CurrentUsage: 512},
		{Name: `D:\pagefile.sys`, AllocatedBaseSize: 8192, CurrentUsage: 1024},
	}

	total, used := sumPageFiles(rows)

	if want := 12288 * mb; total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
	if want := 1536 * mb; used != want {
		t.Errorf("used = %d, want %d", used, want)
	}
}

func TestSumPageFiles_AllocatedButUnused(t *testing.T) {
	rows := []Win32_PageFileUsage{
		{Name: `C:\pagefile.sys`, AllocatedBaseSize: 2048, CurrentUsage: 0},
	}

	total, used := sumPageFiles(rows)

	if want := 2048 * mb; total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
	if used != 0 {
		t.Errorf("used = %d, want 0", used)
	}
}

// A pagefile larger than 4095 MB overflows if the megabyte figure is
// multiplied before being widened to uint64. Every server exceeds that, so
// this guards the conversion order, not an exotic edge case.
func TestSumPageFiles_LargePageFileDoesNotOverflow(t *testing.T) {
	rows := []Win32_PageFileUsage{
		{Name: `C:\pagefile.sys`, AllocatedBaseSize: 65536, CurrentUsage: 40960},
	}

	total, used := sumPageFiles(rows)

	if want := 65536 * mb; total != want {
		t.Errorf("total = %d, want %d (overflow in the MB to byte conversion?)", total, want)
	}
	if want := 40960 * mb; used != want {
		t.Errorf("used = %d, want %d (overflow in the MB to byte conversion?)", used, want)
	}
}

// Asserts invariants only. A host with no pagefile configured is a supported
// configuration, so this deliberately does not require total > 0 -- that
// would fail on a legitimately pagefile-less machine or CI runner.
func TestPageFileUsage_Integration(t *testing.T) {
	total, used, err := pageFileUsage()
	if err != nil {
		t.Fatalf("pageFileUsage failed: %v", err)
	}

	t.Logf("Pagefile: %d bytes total, %d bytes used", total, used)

	if used > total {
		t.Errorf("used (%d) > total (%d)", used, total)
	}
	if total == 0 {
		if used != 0 {
			t.Errorf("no pagefile reported but used = %d", used)
		}
		t.Log("No pagefile configured on this host.")
	}
}

func TestCollect_Integration(t *testing.T) {
	data, err := Collect(context.Background())
	// Collect returns the RAM figures even when the pagefile query fails, so
	// record the error and keep validating rather than bailing out.
	if err != nil {
		t.Errorf("Collect reported an error: %v", err)
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
	t.Logf("Swap/Page: %d bytes total, %d used (%.2f%%)", m.SwapTotal, m.SwapUsed, m.SwapPct)

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

	// The pagefile is a file, not the commit limit, so it can never exceed
	// what the collector reports as allocated.
	if m.SwapUsed > m.SwapTotal {
		t.Errorf("SwapUsed (%d) > SwapTotal (%d)", m.SwapUsed, m.SwapTotal)
	}

	minRAM := uint64(512 * 1024 * 1024)
	if m.Total < minRAM {
		t.Errorf("Total RAM %d seems too low", m.Total)
	}
}

func TestCollect_Consistency(t *testing.T) {
	ctx := context.Background()

	m1, err := Collect(ctx)
	if err != nil {
		t.Errorf("first call reported an error: %v", err)
	}

	m2, err := Collect(ctx)
	if err != nil {
		t.Errorf("second call reported an error: %v", err)
	}

	mem1 := m1[0].(protocol.MemoryMetric)
	mem2 := m2[0].(protocol.MemoryMetric)

	if mem1.Total != mem2.Total {
		t.Errorf("Total changed between calls: %d vs %d", mem1.Total, mem2.Total)
	}

	// A system-managed pagefile can grow, so this is a Log rather than an
	// Error: back-to-back growth is implausible but not impossible, and a
	// flaky failure here would be worse than a note in the output.
	if mem1.SwapTotal != mem2.SwapTotal {
		t.Logf("SwapTotal changed between calls: %d vs %d (system-managed pagefile resize?)", mem1.SwapTotal, mem2.SwapTotal)
	}
}

func BenchmarkCollect(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = Collect(ctx)
	}
}
