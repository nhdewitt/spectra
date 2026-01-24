//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseProcessMemInfoFrom(t *testing.T) {
	input := `
MemTotal:		32806268 kB
MemFree:		18263152 kB
MemAvailable:	27608292 kB
Buffers:		  542380 kB
`

	reader := strings.NewReader(input)

	bytes, err := parseProcessMemInfoFrom(reader)
	if err != nil {
		t.Fatalf("parseMemInfoFrom failed: %v", err)
	}

	expected := uint64(32806268 * 1024)
	if bytes != expected {
		t.Errorf("Expected %d bytes, got %d", expected, bytes)
	}
}

func TestParseProcessMemInfoFrom_NotFound(t *testing.T) {
	input := `
MemFree:		18263152 kB
MemAvailable:	27608292 kB
Buffers:		  542380 kB
`
	reader := strings.NewReader(input)
	_, err := parseProcessMemInfoFrom(reader)
	if err == nil {
		t.Error("expected error when MemTotal not found")
	}
}

func TestParseProcessMemInfoFrom_Empty(t *testing.T) {
	reader := strings.NewReader("")
	_, err := parseProcessMemInfoFrom(reader)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseProcessMemInfoFrom_MalformedValue(t *testing.T) {
	input := `MemTotal:		notanumber kB`
	reader := strings.NewReader(input)
	_, err := parseProcessMemInfoFrom(reader)
	if err == nil {
		t.Error("expected error for malformed value")
	}
}

func TestParsePidStatFrom(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantState string
		wantRSS   uint64
		wantErr   bool
	}{
		{
			name:      "Standard Process",
			input:     "123 (nginx) S 1 123 0 0 0 0 0 0 0 0 10 20 0 0 0 0 0 0 0 0 500 0 0 0 0",
			wantName:  "nginx",
			wantState: "S",
			wantRSS:   500,
			wantErr:   false,
		},
		{
			name:      "Process Name With Spaces",
			input:     "456 (My App) R 1 456 0 0 0 0 0 0 0 0 100 200 0 0 0 0 0 0 0 0 999 0 0 0 0",
			wantName:  "My App",
			wantState: "R",
			wantRSS:   999,
			wantErr:   false,
		},
		{
			name:    "Malformed Line",
			input:   "garbage data without parentheses",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			stat, err := parsePidStatFrom(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if stat.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", stat.Name, tt.wantName)
			}
			if stat.State != tt.wantState {
				t.Errorf("State = %q, want %q", stat.State, tt.wantState)
			}
			if stat.RSSPages != tt.wantRSS {
				t.Errorf("RSSPage = %d, want %d", stat.RSSPages, tt.wantRSS)
			}

			expectedTicks := uint64(0)
			if tt.name == "Standard Process" {
				expectedTicks = 30
			}
			if tt.name == "Process Name With Spaces" {
				expectedTicks = 300
			}

			if stat.TotalTicks != expectedTicks {
				t.Errorf("TotalTicks = %d, want %d", stat.TotalTicks, expectedTicks)
			}
		})
	}
}

func TestParsePidStatFrom_ProcessNameWithParens(t *testing.T) {
	input := "789 (foo (bar)) S 1 789 0 0 0 0 0 0 0 0 50 60 0 0 0 0 0 0 0 0 1000 0 0 0 0"
	reader := strings.NewReader(input)
	stat, err := parsePidStatFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Name != "foo (bar)" {
		t.Errorf("Name = %q, want %q", stat.Name, "foo (bar)")
	}
}

func TestParsePidStatFrom_KernelThread(t *testing.T) {
	// Kernel threads have 0 RSS
	input := "2 (kthreadd) S 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	reader := strings.NewReader(input)
	stat, err := parsePidStatFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Name != "kthreadd" {
		t.Errorf("Name = %q, want %q", stat.Name, "kthreadd")
	}
	if stat.RSSPages != 0 {
		t.Errorf("RSSPages = %d, want 0 for kernel thread", stat.RSSPages)
	}
}

func TestParsePidStatFrom_InsufficientFields(t *testing.T) {
	input := "123 (short) S 1 123"
	reader := strings.NewReader(input)
	_, err := parsePidStatFrom(reader)
	if err == nil {
		t.Error("expected error for insufficient fields")
	}
}

func TestParsePidStatFrom_ZombieProcess(t *testing.T) {
	input := "999 (zombie) Z 1 999 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	reader := strings.NewReader(input)
	stat, err := parsePidStatFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.State != "Z" {
		t.Errorf("State = %q, want %q", stat.State, "Z")
	}
}

func TestParsePidStatFrom_LargeValues(t *testing.T) {
	input := "123 (busy) R 1 123 0 0 0 0 0 0 0 0 999999999 888888888 0 0 0 0 0 0 0 0 50000 0 0 0 0"
	reader := strings.NewReader(input)
	stat, err := parsePidStatFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedTicks := uint64(999999999 + 888888888)
	if stat.TotalTicks != expectedTicks {
		t.Errorf("TotalTicks = %d, want %d", stat.TotalTicks, expectedTicks)
	}
}

func TestCollectProcesses_Integration(t *testing.T) {
	data, err := CollectProcesses(context.Background())
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Returned no metrics")
	}

	listMetric, ok := data[0].(protocol.ProcessListMetric)
	if !ok {
		t.Fatalf("Expected ProcessListMetric, got %T", data[0])
	}

	procs := listMetric.Processes
	t.Logf("Found %d processes", len(procs))
	if len(procs) == 0 {
		t.Error("Process list is empty")
	}

	foundSelf := false
	for _, p := range procs {
		if p.Pid == 1 {
			t.Logf("PID 1: %s (Status: %s, Mem: %d bytes)", p.Name, p.Status, p.MemRSS)
		}
		if p.CPUPercent < 0 {
			t.Errorf("PID %d has negative CPU: %f", p.Pid, p.CPUPercent)
		}
		if p.MemRSS > 0 {
			foundSelf = true
		}
	}

	if !foundSelf {
		t.Error("No processes reported any memory usage.")
	}
}

func TestCollectProcesses_CPUPercentBaseline(t *testing.T) {
	lastProcessStates = make(map[int]processState)

	ctx := context.Background()

	// Baseline - all CPU should be 0
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)
	for _, p := range listMetric.Processes {
		if p.CPUPercent != 0 {
			t.Errorf("PID %d has non-zero CPU on baseline: %f", p.Pid, p.CPUPercent)
		}
	}
}

func TestCollectProcesses_CPUPercentSecondCall(t *testing.T) {
	lastProcessStates = make(map[int]processState)

	ctx := context.Background()

	_, _ = CollectProcesses(ctx)

	time.Sleep(100 * time.Millisecond)

	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)
	t.Logf("Found %d processes on second call", len(listMetric.Processes))

	var totalCPU float64
	for _, p := range listMetric.Processes {
		totalCPU += p.CPUPercent
	}
	t.Logf("Total CPU across all processes: %.2f%%", totalCPU)
}

func TestCollectProcesses_MemPercentRange(t *testing.T) {
	ctx := context.Background()
	data, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	listMetric := data[0].(protocol.ProcessListMetric)
	for _, p := range listMetric.Processes {
		if p.MemPercent < 0 || p.MemPercent > 100 {
			t.Errorf("PID %d has invalid MemPercent: %f", p.Pid, p.MemPercent)
		}
	}
}

func TestProcessStateCleanup(t *testing.T) {
	lastProcessStates = map[int]processState{
		99999999: {lastTicks: 100, lastTime: time.Now()},
	}

	ctx := context.Background()
	_, err := CollectProcesses(ctx)
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	if _, ok := lastProcessStates[99999999]; ok {
		t.Error("old PID not cleaned up")
	}
}

func BenchmarkParseProcessMemInfoFrom(b *testing.B) {
	input := `MemTotal:       32806268 kB
MemFree:        18263152 kB
MemAvailable:   27608292 kB
Buffers:          542380 kB
Cached:          8234567 kB
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseProcessMemInfoFrom(r)
	}
}

func BenchmarkParsePidStatFrom(b *testing.B) {
	input := "12345 (chrome) S 1234 12345 12345 0 -1 4194304 12345 0 123 0 500 200 0 0 20 0 50 0 123456 987654321 25000 18446744073709551615 0 0 0 0 0 0 0 0 0 0 0 0 17 3 0 0 0 0 0"
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parsePidStatFrom(r)
	}
}

func BenchmarkCollectProcesses(b *testing.B) {
	lastProcessStates = make(map[int]processState)
	ctx := context.Background()
	_, _ = CollectProcesses(ctx)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectProcesses(ctx)
	}
}
