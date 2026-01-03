//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseProcUptimeFrom(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantUptime uint64
		wantErr    bool
	}{
		{
			name:       "Standard Uptime",
			input:      "34523.45 234234.22",
			wantUptime: 34523,
			wantErr:    false,
		},
		{
			name:       "Short Uptime",
			input:      "0.00 0.00",
			wantUptime: 0,
			wantErr:    false,
		},
		{
			name:    "Empty File",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Garbage",
			input:   "not_a_number 1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			uptime, bootTime, err := parseProcUptimeFrom(r)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if uptime != tt.wantUptime {
				t.Errorf("Uptime = %d, want %d", uptime, tt.wantUptime)
			}

			now := uint64(time.Now().Unix())
			expectedBoot := now - uptime

			diff := int64(expectedBoot) - int64(bootTime)
			if diff < -1 || diff > 1 {
				t.Errorf("BootTime calculation seems off: Got %d, Expected ~%d (Diff: %d)", bootTime, expectedBoot, diff)
			}
		})
	}
}

func TestCountProcs(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    int
	}{
		{
			name:    "Mixed Content",
			entries: []string{"1", "2", "300", "cpuinfo", "meminfo", "self", "1000"},
			want:    4,
		},
		{
			name:    "Only Numbers",
			entries: []string{"1", "2", "3"},
			want:    3,
		},
		{
			name:    "Only Files",
			entries: []string{"stat", "uptime", "version"},
			want:    0,
		},
		{
			name:    "Empty",
			entries: []string{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countProcs(tt.entries)
			if got != tt.want {
				t.Errorf("countProcs() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseWhoFrom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name: "Two Users",
			input: `
root		tty			2023-10-27 10:00
nhdewitt	pts/0		2023-10-27 10:05 (192.168.1.5)
`,
			want: 2,
		},
		{
			name:  "No Users",
			input: "",
			want:  0,
		},
		{
			name:  "Whitespace Only",
			input: "       		\n 			 ",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got := parseWhoFrom(r)
			if got != tt.want {
				t.Errorf("parseWhoFrom() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCollectSystem_Integration(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Returned no metrics")
	}

	m, ok := data[0].(protocol.SystemMetric)
	if !ok {
		t.Fatalf("Expected SystemMetric, got %T", data[0])
	}

	t.Logf("Uptime: %d seconds", m.Uptime)
	t.Logf("Process Count: %d", m.Processes)
	t.Logf("User Count: %d", m.Users)

	if m.Uptime == 0 {
		t.Error("Uptime reported as 0")
	}
	if m.Processes == 0 {
		t.Error("Process count reported as 0")
	}
}
