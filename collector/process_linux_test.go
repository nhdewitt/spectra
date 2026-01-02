//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/nhdewitt/spectra/metrics"
)

func TestParseMemInfoFrom(t *testing.T) {
	input := `
MemTotal:		32806268 kB
MemFree:		18263152 kB
MemAvailable:	27608292 kB
Buffers:		  542380 kB
`

	reader := strings.NewReader(input)

	bytes, err := parseMemInfoFrom(reader)
	if err != nil {
		t.Fatalf("parseMemInfoFrom failed: %v", err)
	}

	expected := uint64(32806268 * 1024)
	if bytes != expected {
		t.Errorf("Expected %d bytes, got %d", expected, bytes)
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

func TestCollectProcesses_Integration(t *testing.T) {
	data, err := CollectProcesses(context.Background())
	if err != nil {
		t.Fatalf("CollectProcesses failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Returned no metrics")
	}

	listMetric, ok := data[0].(metrics.ProcessListMetric)
	if !ok {
		t.Fatalf("Expected metrics.ProcessListMetric, got %T", data[0])
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
