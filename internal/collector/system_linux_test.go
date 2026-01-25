//go:build !windows

package collector

import (
	"context"
	"os"
	"strconv"
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
			name:       "Large Uptime",
			input:      "8640000.99 12345678.00",
			wantUptime: 8640000,
			wantErr:    false,
		},
		{
			name:       "Fractional Seconds Truncated",
			input:      "100.99 200.00",
			wantUptime: 100,
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
		{
			name:    "Only Whitespace",
			input:   "   \t\n   ",
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

func TestParseProcUptimeFrom_BootTimeConsistency(t *testing.T) {
	input := "1000.00 2000.00"

	r1 := strings.NewReader(input)
	_, bootTime1, _ := parseProcUptimeFrom(r1)

	time.Sleep(10 * time.Millisecond)

	r2 := strings.NewReader(input)
	_, bootTime2, _ := parseProcUptimeFrom(r2)

	diff := int64(bootTime2) - int64(bootTime1)
	if diff < -1 || diff > 1 {
		t.Errorf("BootTime inconsistent between calls: %d vs %d", bootTime1, bootTime2)
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
		{
			name:    "Large PIDs",
			entries: []string{"1", "32768", "65535", "100000"},
			want:    4,
		},
		{
			name:    "Leading Zeros",
			entries: []string{"001", "010", "100"},
			want:    3,
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
	t.Logf("BootTime: %d (Unix)", m.BootTime)
	t.Logf("Process Count: %d", m.Processes)
	t.Logf("User Count: %d", m.Users)

	if m.Uptime == 0 {
		t.Error("Uptime reported as 0")
	}
	if m.Processes == 0 {
		t.Error("Process count reported as 0")
	}
	if m.BootTime == 0 {
		t.Error("BootTime reported as 0")
	}

	now := uint64(time.Now().Unix())
	if m.BootTime > now {
		t.Errorf("BootTime %d is in the future (now: %d)", m.BootTime, now)
	}

	calculated := m.BootTime + m.Uptime
	diff := int64(calculated) - int64(now)
	if diff < -2 || diff > 2 {
		t.Errorf("BootTime + Uptime = %d, expected ~%d", calculated, now)
	}
}

func TestCollectSystem_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectSystem(ctx)
	if err != nil {
		t.Logf("CollectSystem with cancelled context: %v", err)
	}
}

func TestCollectSystem_ProcessCountMatchesProc(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}

	m, ok := data[0].(protocol.SystemMetric)
	if !ok {
		t.Fatalf("expected SystemMetric, got %T", data[0])
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("failed to read /proc: %v", err)
	}

	actualCount := 0
	for _, e := range entries {
		if e.IsDir() {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				actualCount++
			}
		}
	}

	diff := m.Processes - actualCount
	if diff < -5 || diff > 5 {
		t.Errorf("process count mismatch: reported %d, actual %d", m.Processes, actualCount)
	}
}

func BenchmarkParseProcUptimeFrom(b *testing.B) {
	input := "123456.78 987654.32"
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _, _ = parseProcUptimeFrom(r)
	}
}

func BenchmarkCountProcs(b *testing.B) {
	// Simulate typical /proc with ~200 processes
	entries := make([]string, 250)
	for i := 0; i < 200; i++ {
		entries[i] = strconv.Itoa(i + 1)
	}
	// Add non-numeric entries
	nonNumeric := []string{
		"cpuinfo", "meminfo", "stat", "uptime", "version",
		"filesystems", "mounts", "net", "sys", "bus", "driver", "fs",
		"irq", "kernel", "self", "thread-self", "tty", "crypto",
		"diskstats", "kallsyms", "kmsg", "kpagecount", "loadavg",
		"locks", "mdstat", "misc", "modules", "partitions", "sched_debug",
		"schedstat", "slabinfo", "softirqs", "stat", "swaps", "timer_list",
		"timer_stats", "vmstat", "zoneinfo", "buddyinfo", "cgroups",
		"cmdline", "consoles", "devices", "dma", "execdomains", "fb",
		"interrupts", "iomem", "ioports", "key-users",
	}
	copy(entries[200:], nonNumeric)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = countProcs(entries)
	}
}

func BenchmarkParseWhoFrom_Empty(b *testing.B) {
	input := ""
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_ = parseWhoFrom(r)
	}
}

func BenchmarkParseWhoFrom_MultipleUsers(b *testing.B) {
	input := `root     tty1         2023-10-27 10:00
user1    pts/0        2023-10-27 10:05 (192.168.1.100)
user2    pts/1        2023-10-27 10:10 (10.0.0.50)
user3    pts/2        2023-10-27 10:15 (172.16.0.1)
user4    pts/3        2023-10-27 10:20 (192.168.1.200)`

	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_ = parseWhoFrom(r)
	}
}

func BenchmarkCollectSystem(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectSystem(ctx)
	}
}
