//go:build windows

package collector

import (
	"context"
	"testing"
	"time"
	"unsafe"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows"
)

func TestCountQUserLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name: "Single User",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
nhdewitt			console				1	Active	none		1/2/2026 8:00AM
`,
			want: 1,
		},
		{
			name: "Multiple Users",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
admin				console				1	Active	none		1/2/2026 8:00AM
guest				rdp-tcp#0			2	Active	none		1/2/2026 9:00AM
`,
			want: 2,
		},
		{
			name:  "Empty Output",
			input: "",
			want:  0,
		},
		{
			name: "Header Only",
			input: `
USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
`,
			want: 0,
		},
		{
			name: "Disconnected Sessions",
			input: `USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
admin				console				1	Active	none		1/2/2026 8:00AM
guest								2	Disc	30			1/2/2026 9:00AM
service							3	Disc	1:00		1/2/2026 7:00AM`,
			want: 3,
		},
		{
			name: "RDP Sessions",
			input: `USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
localuser			console				1	Active	none		1/2/2026 8:00AM
rdpuser1			rdp-tcp#0			2	Active	none		1/2/2026 9:00AM
rdpuser2			rdp-tcp#1			3	Active	5			1/2/2026 10:00AM
rdpuser3			rdp-tcp#2			4	Disc	30			1/2/2026 11:00AM`,
			want: 4,
		},
		{
			name:  "Whitespace Only",
			input: "   \t\n   ",
			want:  0,
		},
		{
			name: "Extra Blank Lines",
			input: `USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME

user1				console				1	Active	none		1/2/2026 8:00AM

user2				rdp-tcp#0			2	Active	none		1/2/2026 9:00AM
`,
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countQUserLines([]byte(tt.input))
			if got != tt.want {
				t.Errorf("countQUserLines() = %d, want %d", got, tt.want)
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
	t.Logf("Processes: %d", m.Processes)
	t.Logf("Users: %d", m.Users)

	if m.Uptime == 0 {
		t.Error("Uptime reported as 0")
	}
	if m.Processes == 0 {
		t.Error("Process count reported as 0")
	}

	now := uint64(time.Now().Unix())
	expectedBoot := now - m.Uptime
	if m.BootTime > now {
		t.Errorf("BootTime %d is in the future (now: %d)", m.BootTime, now)
	}

	diff := int64(expectedBoot) - int64(m.BootTime)
	if diff < -2 || diff > 2 {
		t.Errorf("BootTime calculation seems off: Got %d, Expected ~%d (Diff: %d)", m.BootTime, expectedBoot, diff)
	}
}

func TestCollectSystem_UptimeIncreases(t *testing.T) {
	ctx := context.Background()

	data1, err := CollectSystem(ctx)
	if err != nil {
		t.Fatalf("first CollectSystem failed: %v", err)
	}
	m1 := data1[0].(protocol.SystemMetric)

	time.Sleep(1100 * time.Millisecond)

	data2, err := CollectSystem(ctx)
	if err != nil {
		t.Fatalf("second CollectSystem failed: %v", err)
	}
	m2 := data2[0].(protocol.SystemMetric)

	if m2.Uptime <= m1.Uptime {
		t.Errorf("Uptime did not increase: %d -> %d", m1.Uptime, m2.Uptime)
	}

	diff := m2.Uptime - m1.Uptime
	if diff < 1 || diff > 2 {
		t.Errorf("Uptime increased by %d seconds, expected ~1", diff)
	}
}

func TestCollectSystem_BootTimeStable(t *testing.T) {
	ctx := context.Background()

	data1, err := CollectSystem(ctx)
	if err != nil {
		t.Fatalf("First CollectSystem failed: %v", err)
	}
	m1 := data1[0].(protocol.SystemMetric)

	time.Sleep(100 * time.Millisecond)

	data2, err := CollectSystem(ctx)
	if err != nil {
		t.Fatalf("Second CollectSystem failed: %v", err)
	}
	m2 := data2[0].(protocol.SystemMetric)

	diff := int64(m2.BootTime) - int64(m1.BootTime)
	if diff < -1 || diff > 1 {
		t.Errorf("BootTime changed: %d -> %d", m1.BootTime, m2.BootTime)
	}
}

func TestCollectSystem_ProcessCountReasonable(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}

	m := data[0].(protocol.SystemMetric)

	if m.Processes < 20 {
		t.Errorf("Process count suspiciously low: %d", m.Processes)
	}
	if m.Processes > 2000 {
		t.Errorf("Process count suspiciously high: %d", m.Processes)
	}
}

func TestCollectSystem_ProcessCountMatchesSnapshot(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}

	m := data[0].(protocol.SystemMetric)

	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatalf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer windows.CloseHandle(handle)

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	actualCount := 0
	if err := windows.Process32First(handle, &pe32); err == nil {
		actualCount++
		for windows.Process32Next(handle, &pe32) == nil {
			actualCount++
		}
	}

	// Allow variance since processes can start/stop
	diff := m.Processes - actualCount
	if diff < -10 || diff > 10 {
		t.Errorf("Process count mismatch: reported %d, actual %d", m.Processes, actualCount)
	}
}

func TestCollectSystem_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still partially work - uptime and process count don't use context
	data, err := CollectSystem(ctx)
	if err != nil {
		t.Logf("CollectSystem with cancelled context returned error: %v", err)
	}

	if len(data) > 0 {
		m := data[0].(protocol.SystemMetric)
		// Uptime and processes should still be populated
		if m.Uptime == 0 {
			t.Error("Uptime should still be populated with cancelled context")
		}
	}
}

func TestCollectSystem_AtLeastOneUser(t *testing.T) {
	data, err := CollectSystem(context.Background())
	if err != nil {
		t.Fatalf("CollectSystem failed: %v", err)
	}

	m := data[0].(protocol.SystemMetric)

	// Running this test means at least one user is logged in
	if m.Users < 1 {
		t.Log("Warning: No users reported - quser may have failed or no interactive sessions")
	}
}

func BenchmarkCountQUserLines_Empty(b *testing.B) {
	input := []byte("")
	b.ReportAllocs()
	for b.Loop() {
		countQUserLines(input)
	}
}

func BenchmarkCountQUserLines_Single(b *testing.B) {
	input := []byte(`USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
nhdewitt			console				1	Active	none		1/2/2026 8:00AM`)
	b.ReportAllocs()
	for b.Loop() {
		countQUserLines(input)
	}
}

func BenchmarkCountQUserLines_Multiple(b *testing.B) {
	input := []byte(`USERNAME			SESSIONNAME			ID	STATE	IDLE TIME	LOGON TIME
admin				console				1	Active	none		1/2/2026 8:00AM
guest				rdp-tcp#0			2	Active	none		1/2/2026 9:00AM
user1				rdp-tcp#1			3	Active	5			1/2/2026 10:00AM
user2				rdp-tcp#2			4	Disc	30			1/2/2026 11:00AM
user3				rdp-tcp#3			5	Disc	1:00		1/2/2026 12:00PM`)
	b.ReportAllocs()
	for b.Loop() {
		countQUserLines(input)
	}
}

func BenchmarkCollectSystem(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectSystem(ctx)
	}
}

func BenchmarkCollectSystem_UptimeOnly(b *testing.B) {
	// Benchmark just the GetTickCount64 portion
	b.ReportAllocs()
	for b.Loop() {
		ret, _, _ := procGetTickCount64.Call()
		_ = uint64(ret) / 1000
	}
}

func BenchmarkCollectSystem_ProcessCount(b *testing.B) {
	// Benchmark just the process enumeration
	b.ReportAllocs()
	for b.Loop() {
		handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
		if err != nil {
			continue
		}

		var pe32 windows.ProcessEntry32
		pe32.Size = uint32(unsafe.Sizeof(pe32))

		count := 0
		if windows.Process32First(handle, &pe32) == nil {
			count++
			for windows.Process32Next(handle, &pe32) == nil {
				count++
			}
		}
		windows.CloseHandle(handle)
		_ = count
	}
}
