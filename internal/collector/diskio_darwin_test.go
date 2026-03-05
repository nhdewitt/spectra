//go:build darwin

package collector

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseIoregOutput(t *testing.T) {
	sample := []byte(`+-o IOBlockStorageDriver  <class IOBlockStorageDriver, id 0x10000072b, registered, matched, active, busy 0 (122 ms), retain 8>
  | {
  |   "Statistics" = {"Operations (Write)"=1006613,"Latency Time (Write)"=0,"Bytes (Read)"=30948818944,"Errors (Write)"=0,"Total Time (Read)"=640948223045,"Latency Time (Read)"=0,"Retries (Read)"=0,"Errors (Read)"=0,"Total Time (Write)"=67068804165,"Bytes (Write)"=14221045760,"Operations (Read)"=2328858,"Retries (Write)"=0}
  | }
  | 
  +-o APPLE SSD AP0128Q Media  <class IOMedia, id 0x10000072d, registered, matched, active, busy 0 (136 ms), retain 12>
`)

	result := parseIoregOutput(sample)

	if len(result) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(result))
	}

	d, ok := result["APPLE SSD AP0128Q"]
	if !ok {
		t.Fatalf("missing 'APPLE SSD AP0128Q', got keys: %v", mapKeys(result))
	}

	if d.ReadBytes != 30948818944 {
		t.Errorf("ReadBytes = %d, want 30948818944", d.ReadBytes)
	}
	if d.WriteBytes != 14221045760 {
		t.Errorf("WriteBytes = %d, want 14221045760", d.WriteBytes)
	}
	if d.ReadOps != 2328858 {
		t.Errorf("ReadOps = %d, want 2328858", d.ReadOps)
	}
	if d.WriteOps != 1006613 {
		t.Errorf("WriteOps = %d, want 1006613", d.WriteOps)
	}
	// 640948223045 ns → 640948 ms
	if d.ReadTime != 640948 {
		t.Errorf("ReadTime = %d ms, want 640948", d.ReadTime)
	}
	// 67068804165 ns → 67068 ms
	if d.WriteTime != 67068 {
		t.Errorf("WriteTime = %d ms, want 67068", d.WriteTime)
	}
}

func mapKeys(m map[string]DiskIORaw) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
}

func TestParseIoregOutputMultipleDisks(t *testing.T) {
	sample := []byte(`+-o IOBlockStorageDriver  <class IOBlockStorageDriver, id 0x100000345, ...>
  | {
  |   "Statistics" = {"Bytes (Read)"=100000,"Bytes (Write)"=200000,"Operations (Read)"=50,"Operations (Write)"=100,"Total Time (Read)"=1000000,"Total Time (Write)"=2000000}
  | }
  |
  +-o APPLE SSD AP0128Q Media  <class IOMedia, id 0x100000346, ...>

+-o IOBlockStorageDriver  <class IOBlockStorageDriver, id 0x100000400, ...>
  | {
  |   "Statistics" = {"Bytes (Read)"=300000,"Bytes (Write)"=400000,"Operations (Read)"=150,"Operations (Write)"=200,"Total Time (Read)"=3000000,"Total Time (Write)"=4000000}
  | }
  |
  +-o Generic USB Flash Media  <class IOMedia, id 0x100000401, ...>
`)

	result := parseIoregOutput(sample)

	if len(result) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(result))
	}

	if _, ok := result["APPLE SSD AP0128Q"]; !ok {
		t.Error("missing APPLE SSD AP0128Q")
	}
	if _, ok := result["Generic USB Flash"]; !ok {
		t.Error("missing Generic USB Flash")
	}

	if result["Generic USB Flash"].ReadBytes != 300000 {
		t.Errorf("USB ReadBytes = %d, want 300000", result["Generic USB Flash"].ReadBytes)
	}
}

func TestParseIoregOutputEmpty(t *testing.T) {
	result := parseIoregOutput([]byte(""))
	if len(result) != 0 {
		t.Errorf("expected 0 disks, got %d", len(result))
	}
}

func TestParseIoregOutputNoStats(t *testing.T) {
	sample := []byte(`+-o IOBlockStorageDriver  <class IOBlockStorageDriver, id 0x100000345, ...>
  | {
  | }
  |
  +-o APPLE SSD AP0128Q Media  <class IOMedia, id 0x100000346, ...>
`)

	result := parseIoregOutput(sample)

	if len(result) != 0 {
		t.Errorf("expected 0 disks without stats, got %d", len(result))
	}
}

func TestParseStatsDict(t *testing.T) {
	line := `  |   "Statistics" = {"Operations (Write)"=1006613,"Bytes (Read)"=30948818944,"Total Time (Read)"=640948223045,"Bytes (Write)"=14221045760,"Operations (Read)"=2328858,"Total Time (Write)"=67068804165}`

	stats := parseStatsDict(line)

	tests := []struct {
		key  string
		want uint64
	}{
		{"Bytes (Read)", 30948818944},
		{"Bytes (Write)", 14221045760},
		{"Operations (Read)", 2328858},
		{"Operations (Write)", 1006613},
		{"Total Time (Read)", 640948223045},
		{"Total Time (Write)", 67068804165},
	}

	for _, tc := range tests {
		got, ok := stats[tc.key]
		if !ok {
			t.Errorf("missing key %q", tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("stats[%q] = %d, want %d", tc.key, got, tc.want)
		}
	}
}

func TestParseStatsDictEmpty(t *testing.T) {
	stats := parseStatsDict(`"Statistics" = {}`)
	if len(stats) != 0 {
		t.Errorf("expected empty map, got %d entries", len(stats))
	}
}

func TestParseIOMediaName(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"  +-o APPLE SSD AP0128Q Media  <class IOMedia, id 0x10000072d, ...>", "APPLE SSD AP0128Q"},
		{"  +-o Generic USB Flash Media  <class IOMedia, id 0x100000401, ...>", "Generic USB Flash"},
		{"  +-o disk0  <class IOMedia, id 0x100000346, ...>", "disk0"},
		{"  +-o Samsung SSD 870 EVO 1TB Media  <class IOMedia, ...>", "Samsung SSD 870 EVO 1TB"},
		{"no match here", ""},
		{"  +-o <class IOMedia>", ""},
	}

	for _, tc := range tests {
		got := parseIOMediaName(tc.line)
		if got != tc.want {
			t.Errorf("parseIOMediaName(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestBuildDiskIOMetric(t *testing.T) {
	prev := DiskIORaw{
		ReadBytes:  1000,
		WriteBytes: 2000,
		ReadOps:    10,
		WriteOps:   20,
		ReadTime:   100,
		WriteTime:  200,
	}
	curr := DiskIORaw{
		ReadBytes:  6000,
		WriteBytes: 12000,
		ReadOps:    60,
		WriteOps:   120,
		ReadTime:   150,
		WriteTime:  250,
	}

	m := buildDiskIOMetric("APPLE SSD AP0128Q", curr, prev, 5.0)

	if m.Device != "APPLE SSD AP0128Q" {
		t.Errorf("Device = %q, want APPLE SSD AP0128Q", m.Device)
	}
	if m.ReadBytes != 1000 {
		t.Errorf("ReadBytes = %d, want 1000", m.ReadBytes)
	}
	if m.WriteBytes != 2000 {
		t.Errorf("WriteBytes = %d, want 2000", m.WriteBytes)
	}
	if m.ReadTime != 50 {
		t.Errorf("ReadTime = %d, want 50", m.ReadTime)
	}
	if m.WriteTime != 50 {
		t.Errorf("WriteTime = %d, want 50", m.WriteTime)
	}
}

func TestReadDiskIOStats_Integration(t *testing.T) {
	ctx := context.Background()
	stats, err := readDiskIOStats(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) == 0 {
		t.Fatal("expected at least one disk")
	}

	for name, s := range stats {
		t.Logf("%s: read=%d write=%d rops=%d wops=%d rtime=%dms wtime=%dms",
			name, s.ReadBytes, s.WriteBytes, s.ReadOps, s.WriteOps, s.ReadTime, s.WriteTime)

		if s.ReadBytes == 0 && s.WriteBytes == 0 {
			t.Errorf("%s: both read and write bytes are 0", name)
		}
	}
}

func TestCollectDiskIO_FirstSampleNil(t *testing.T) {
	lastDiskIORaw = nil
	lastDiskIOTime = time.Time{}

	ctx := context.Background()

	metrics, err := CollectDiskIO(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if metrics != nil {
		t.Error("first sample should return nil")
	}
}

func TestCollectDiskIO_SecondSample(t *testing.T) {
	lastDiskIORaw = nil
	lastDiskIOTime = time.Time{}

	ctx := context.Background()

	// Prime
	_, err := CollectDiskIO(ctx)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	// Collect
	metrics, err := CollectDiskIO(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if metrics == nil {
		t.Fatal("second sample returned nil")
	}

	for _, m := range metrics {
		dm := m.(protocol.DiskIOMetric)
		t.Logf("%s: read=%d/s write=%d/s", dm.Device, dm.ReadBytes, dm.WriteBytes)
	}
}

func BenchmarkReadDiskIOStats(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_, _ = readDiskIOStats(ctx)
	}
}

func BenchmarkParseIoregOutput(b *testing.B) {
	ctx := context.Background()
	out, err := exec.CommandContext(
		ctx, "ioreg", "-d", "3", "-c",
		"IOBlockStorageDriver", "-r", "-w", "0",
	).Output()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = parseIoregOutput(out)
	}
}

func BenchmarkParseStatsDict(b *testing.B) {
	line := `.  |   "Statistics" = {"Operations (Write)"=1006613,"Latency Time (Write)"=0,"Bytes (Read)"=30948818944,"Errors (Write)"=0,"Total Time (Read)"=640948223045,"Latency Time (Read)"=0,"Retries (Read)"=0,"Errors (Read)"=0,"Total Time (Write)"=67068804165,"Bytes (Write)"=14221045760,"Operations (Read)"=2328858,"Retries (Write)"=0}`

	b.ResetTimer()
	for b.Loop() {
		_ = parseStatsDict(line)
	}
}

func BenchmarkBuildDiskIOMetric(b *testing.B) {
	prev := DiskIORaw{
		ReadBytes:  1000,
		WriteBytes: 2000,
		ReadOps:    10,
		WriteOps:   20,
		ReadTime:   100,
		WriteTime:  200,
	}

	cur := DiskIORaw{
		ReadBytes:  6000,
		WriteBytes: 12000,
		ReadOps:    60,
		WriteOps:   120,
		ReadTime:   150,
		WriteTime:  250,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = buildDiskIOMetric("APPLE SSD AP0128Q", cur, prev, 5.0)
	}
}
