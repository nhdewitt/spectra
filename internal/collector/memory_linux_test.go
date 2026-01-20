//go:build !windows
// +build !windows

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseMemInfoFrom_Valid(t *testing.T) {
	input := `
MemTotal:		16307664 kB
MemFree:		 1000000 kB
MemAvailable:	 8000000 kB
Buffers:		  500000 kB
Cached:			 2000000 kB
SwapTotal:		 4000000 kB
SwapFree:		 3000000 kB
Dirty:			     100 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTotal := uint64(16307664 * 1024)
	expectedAvail := uint64(8000000 * 1024)
	expectedSwapTotal := uint64(4000000 * 1024)
	expectedSwapFree := uint64(3000000 * 1024)

	if raw.Total != expectedTotal {
		t.Errorf("MemTotal: got %d, want %d", raw.Total, expectedTotal)
	}
	if raw.Available != expectedAvail {
		t.Errorf("MemAvailable: got %d, want %d", raw.Available, expectedAvail)
	}
	if raw.SwapTotal != expectedSwapTotal {
		t.Errorf("SwapTotal: got %d, want %d", raw.SwapTotal, expectedSwapTotal)
	}
	if raw.SwapFree != expectedSwapFree {
		t.Errorf("SwapFree: got %d, want %d", raw.SwapFree, expectedSwapFree)
	}
}

func TestParseMemInfoFrom_MissingFields(t *testing.T) {
	input := `
MemTotal:		16307664 kB
MemAvailable:	 8000000 kB
SwapTotal:		 4000000 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	_, err := parseMemInfoFrom(r)
	if err == nil {
		t.Fatal("expected error due to missing SwapFree, got nil")
	}
	if !strings.Contains(err.Error(), "missing fields") {
		t.Errorf("expected 'missing fields' error, got %v", err)
	}
}

func TestParseMemInfoFrom_Malformed(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name: "Bad Number",
			input: `
MemTotal:		NotANumber kB
MemAvailable:	100 kB
SwapTotal:		100 kB
SwapFree:		100 kB`,
			expectErr: true,
		},
		{
			name: "Empty Line Handling",
			input: `
MemTotal:		100 kB
MemAvailable:	100 kB

SwapTotal:		100 kB
SwapFree:		100 kB`,
			expectErr: false,
		},
		{
			name: "Missing Colon",
			input: `
MemTotal		100 kB
MemAvailable:	100 kB
SwapTotal:		100 kB
SwapFree:		100 kB`,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(strings.TrimSpace(tt.input))
			_, err := parseMemInfoFrom(r)

			if tt.expectErr != (err != nil) {
				t.Errorf("malformed line (%s) expected err (%t), got %v", tt.name, tt.expectErr, err)
			}
		})
	}
}

func TestParseMemInfoFrom_EmptyInput(t *testing.T) {
	r := strings.NewReader("")
	_, err := parseMemInfoFrom(r)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "missing fields") {
		t.Errorf("expected 'missing fields' error, got %v", err)
	}
}

func TestParseMemInfoFrom_ZeroValues(t *testing.T) {
	input := `
MemTotal:		0 kB
MemAvailable:	0 kB
SwapTotal:		0 kB
SwapFree:		0 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total != 0 || raw.Available != 0 || raw.SwapTotal != 0 || raw.SwapFree != 0 {
		t.Errorf("expected all zeros, got %+v", raw)
	}
}

func TestParseMemInfoFrom_NoSwap(t *testing.T) {
	input := `
MemTotal:		16307664 kB
MemAvailable:	 8000000 kB
SwapTotal:			   0 kB
SwapFree:			   0 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.SwapTotal != 0 || raw.SwapFree != 0 {
		t.Errorf("expected zero swap, got total=%d free =%d", raw.SwapTotal, raw.SwapFree)
	}
}

func TestParseMemInfoFrom_LargeValues(t *testing.T) {
	input := `
MemTotal:		1073741824 kB
MemAvailable:	 536870912 kB
SwapTotal:		 134217728 kB
SwapFree:		 134217728 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTotal := uint64(1073741824 * 1024)
	if raw.Total != expectedTotal {
		t.Errorf("expected %d, got %d", expectedTotal, raw.Total)
	}
}

func TestParseMemInfoFrom_ExtraWhitespace(t *testing.T) {
	input := `
MemTotal:					16307664 kB
MemAvailable:					8000000 kB
SwapTotal:						4000000 kB
SwapFree:						3000000 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total != 16307664*1024 {
		t.Errorf("whitespace handling failed: got %d", raw.Total)
	}
}

func TestParseMemInfoFrom_DifferentFieldOrder(t *testing.T) {
	input := `
SwapFree:		 3000000 kB
MemAvailable:	 8000000 kB
SwapTotal:		 4000000 kB
MemTotal:		16307664 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total != 16307664*1024 {
		t.Errorf("field order handling failed: got %d", raw.Total)
	}
}

func TestParseMemInfoFrom_DuplicateFields(t *testing.T) {
	input := `
MemTotal:		16307664 kB
MemAvailable:	 8000000 kB
SwapTotal:		 4000000 kB
SwapFree:		 3000000 kB
MemTotal:		99999999 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total != 16307664*1024 {
		// Should stop after finding all 4 fields, not continue on to duplicate field
		t.Logf("Note: duplicate fields use value %d", raw.Total)
	}
}

func TestParseMemInfoFrom_DuplicateFieldsInDifferentOrder(t *testing.T) {
	input := `
MemTotal:		16307664 kB
MemTotal:		99999999 kB
MemAvailable:	 8000000 kB
SwapTotal:		 4000000 kB
SwapFree:		 3000000 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw.Total != 16307664*1024 {
		// Should ignore the duplicate field even if it hasn't found all 4 fields
		t.Logf("Note: duplicate fields in a different order use value %d", raw.Total)
	}
}

func TestCollectMemory_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m, ok := metrics[0].(protocol.MemoryMetric)
	if !ok {
		t.Fatalf("expected MemoryMetric, got %T", metrics[0])
	}

	if m.Total == 0 {
		t.Error("Total should not be zero")
	}
	if m.Available > m.Total {
		t.Errorf("Available (%d) > Total (%d)", m.Available, m.Total)
	}
	if m.Used > m.Total {
		t.Errorf("Used (%d) > Total (%d)", m.Used, m.Total)
	}
	if m.UsedPct < 0 || m.UsedPct > 100 {
		t.Errorf("UsedPct out of range: %f", m.UsedPct)
	}
	if m.SwapPct < 0 || m.SwapPct > 100 {
		t.Errorf("SwapPct out of range: %f", m.SwapPct)
	}

	t.Logf("Memory: Total=%d Available=%d Used=%d (%.1f%%)", m.Total, m.Available, m.Used, m.UsedPct)
	t.Logf("Swap: Total=%d Used=%d (%.1f%%)", m.SwapTotal, m.SwapUsed, m.SwapPct)
}

func TestCollectMemory_UsedCalculation(t *testing.T) {
	// Verify Used = Total - Available
	input := `
MemTotal:		16000000 kB
MemAvailable:	10000000 kB
SwapTotal:		 4000000 kB
SwapFree:		 1000000 kB
`
	r := strings.NewReader(strings.TrimSpace(input))
	raw, err := parseMemInfoFrom(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	used := raw.Total - raw.Available
	expectedUsed := uint64(6000000 * 1024)
	if used != expectedUsed {
		t.Errorf("used calculation: got %d, want %d", used, expectedUsed)
	}

	swapUsed := raw.SwapTotal - raw.SwapFree
	expectedSwapUsed := uint64(3000000 * 1024)
	if swapUsed != expectedSwapUsed {
		t.Errorf("swap used calculation: got %d, want %d", swapUsed, expectedSwapUsed)
	}
}

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectMemory(ctx)
	}
}

func BenchmarkParseMemInfoFrom_Minimal(b *testing.B) {
	input := `
MemTotal:       16307664 kB
MemAvailable:    8000000 kB
SwapTotal:       4000000 kB
SwapFree:        3000000 kB
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseMemInfoFrom(r)
	}
}

func BenchmarkParseMemInfoFrom_Realistic(b *testing.B) {
	input := `
MemTotal:       32768000 kB
MemFree:         5000000 kB
MemAvailable:   12000000 kB
Buffers:          100000 kB
Cached:          4000000 kB
SwapCached:            0 kB
Active:         10000000 kB
Inactive:        5000000 kB
Active(anon):    8000000 kB
Inactive(anon):  1000000 kB
Active(file):    2000000 kB
Inactive(file):  4000000 kB
Unevictable:           0 kB
Mlocked:               0 kB
SwapTotal:       8000000 kB
SwapFree:        7500000 kB
Dirty:               100 kB
Writeback:             0 kB
AnonPages:       9000000 kB
Mapped:           500000 kB
Shmem:            100000 kB
KReclaimable:     200000 kB
Slab:             300000 kB
SReclaimable:     150000 kB
SUnreclaim:       150000 kB
KernelStack:       20000 kB
PageTables:        50000 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:    24000000 kB
Committed_AS:   15000000 kB
VmallocTotal:   34359738367 kB
VmallocUsed:       50000 kB
VmallocChunk:          0 kB
Percpu:            10000 kB
HardwareCorrupted:     0 kB
AnonHugePages:         0 kB
ShmemHugePages:        0 kB
ShmemPmdMapped:        0 kB
FileHugePages:         0 kB
FilePmdMapped:         0 kB
HugePages_Total:       0
HugePages_Free:        0
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
Hugetlb:               0 kB
DirectMap4k:      200000 kB
DirectMap2M:     8000000 kB
DirectMap1G:    26000000 kB
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseMemInfoFrom(r)
	}
}
