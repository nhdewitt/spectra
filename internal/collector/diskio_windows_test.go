//go:build windows

package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func mockTime(tb testing.TB) *time.Time {
	tb.Helper()
	fakeTime := time.Now()
	nowFunc = func() time.Time { return fakeTime }
	tb.Cleanup(func() { nowFunc = time.Now })
	return &fakeTime
}

func TestFormatDeviceName(t *testing.T) {
	tests := []struct {
		name      string
		idx       uint32
		driveInfo MountInfo
		letterMap map[uint32][]string
		want      string
	}{
		{
			name:      "single drive letter",
			idx:       0,
			driveInfo: MountInfo{Model: "Samsung SSD"},
			letterMap: map[uint32][]string{0: {"C:"}},
			want:      "Samsung SSD (C:)",
		},
		{
			name:      "multiple drive letters",
			idx:       1,
			driveInfo: MountInfo{Model: "Samsung SSD"},
			letterMap: map[uint32][]string{1: {"D:", "E:"}},
			want:      "Samsung SSD (D:, E:)",
		},
		{
			name:      "fallback to model when no letter",
			idx:       2,
			driveInfo: MountInfo{Model: "Samsung SSD 990 PRO"},
			letterMap: map[uint32][]string{},
			want:      "Samsung SSD 990 PRO",
		},
		{
			name:      "fallback to PhysicalDrive when no letter or model",
			idx:       3,
			driveInfo: MountInfo{},
			letterMap: map[uint32][]string{},
			want:      "PhysicalDrive3",
		},
		{
			name:      "letter map exists but empty slice",
			idx:       0,
			driveInfo: MountInfo{Model: "WD Blue"},
			letterMap: map[uint32][]string{0: {}},
			want:      "WD Blue",
		},
		{
			name:      "different drive index in map",
			idx:       5,
			driveInfo: MountInfo{Model: "Seagate"},
			letterMap: map[uint32][]string{0: {"C:"}, 1: {"D:"}},
			want:      "Seagate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDeviceName(tt.idx, tt.driveInfo, tt.letterMap)
			if got != tt.want {
				t.Errorf("formatDeviceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectDiskIO_EmptyCache(t *testing.T) {
	lastDiskPerf = nil

	cache := &DriveCache{
		AllowedDrives:  make(map[uint32]MountInfo),
		DriveLetterMap: make(map[uint32][]string),
	}

	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty cache, got %v", result)
	}
}

func TestCollectDiskIO_BaselineCollection(t *testing.T) {
	lastDiskPerf = nil

	// Save original and restore after test
	origGetter := getDrivePerf
	defer func() { getDrivePerf = origGetter }()

	getDrivePerf = func(idx uint32) (diskPerformance, error) {
		return diskPerformance{
			BytesRead:    1000,
			BytesWritten: 2000,
			ReadCount:    10,
			WriteCount:   20,
		}, nil
	}

	cache := &DriveCache{
		AllowedDrives:  map[uint32]MountInfo{0: {Model: "TestDrive"}},
		DriveLetterMap: map[uint32][]string{0: {"C:"}},
	}

	// First call establishes baseline
	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on baseline collection, got %v", result)
	}
}

func TestCollectDiskIO_RateCalculation(t *testing.T) {
	lastDiskPerf = nil
	fakeTime := mockTime(t)

	origGetter := getDrivePerf
	defer func() { getDrivePerf = origGetter }()

	callCount := 0
	getDrivePerf = func(idx uint32) (diskPerformance, error) {
		callCount++
		if callCount == 1 {
			// Baseline
			return diskPerformance{
				BytesRead:    1000,
				BytesWritten: 2000,
				ReadCount:    10,
				WriteCount:   20,
				ReadTime:     100,
				WriteTime:    200,
				QueueDepth:   0,
			}, nil
		}
		// Second call - simulate activity
		return diskPerformance{
			BytesRead:    1000 + 5000,  // +5000 bytes
			BytesWritten: 2000 + 10000, // +10000 bytes
			ReadCount:    10 + 25,      // +25 ops
			WriteCount:   20 + 50,      // +50 ops
			ReadTime:     100 + 500,
			WriteTime:    200 + 1000,
			QueueDepth:   2,
		}, nil
	}

	cache := &DriveCache{
		AllowedDrives:  map[uint32]MountInfo{0: {Model: "TestDrive"}},
		DriveLetterMap: map[uint32][]string{0: {"C:"}},
	}

	// Baseline
	_, _ = CollectDiskIO(context.Background(), cache)

	// Advance time by 5 seconds
	*fakeTime = fakeTime.Add(5 * time.Second)

	// Actual collection
	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(result))
	}

	metric, ok := result[0].(protocol.DiskIOMetric)
	if !ok {
		t.Fatalf("expected DiskIOMetric, got %T", result[0])
	}

	// Rates = delta / timeDelta (5.0)
	expectedReadBytes := uint64(5000 / 5.0)   // 1000
	expectedWriteBytes := uint64(10000 / 5.0) // 2000
	expectedReadOps := uint64(25 / 5.0)       // 5
	expectedWriteOps := uint64(50 / 5.0)      // 10

	if metric.Device != "TestDrive (C:)" {
		t.Errorf("Device = %q, want %q", metric.Device, "C:")
	}
	if metric.ReadBytes != expectedReadBytes {
		t.Errorf("ReadBytes = %d, want %d", metric.ReadBytes, expectedReadBytes)
	}
	if metric.WriteBytes != expectedWriteBytes {
		t.Errorf("WriteBytes = %d, want %d", metric.WriteBytes, expectedWriteBytes)
	}
	if metric.ReadOps != expectedReadOps {
		t.Errorf("ReadOps = %d, want %d", metric.ReadOps, expectedReadOps)
	}
	if metric.WriteOps != expectedWriteOps {
		t.Errorf("WriteOps = %d, want %d", metric.WriteOps, expectedWriteOps)
	}
	if metric.ReadTime != 500 {
		t.Errorf("ReadTime = %d, want %d", metric.ReadTime, 500)
	}
	if metric.WriteTime != 1000 {
		t.Errorf("WriteTime = %d, want %d", metric.WriteTime, 1000)
	}
	if metric.InProgress != 2 {
		t.Errorf("InProgress = %d, want %d", metric.InProgress, 2)
	}
}

func TestCollectDiskIO_MultipleDrives(t *testing.T) {
	lastDiskPerf = nil
	fakeTime := mockTime(t)

	origGetter := getDrivePerf
	defer func() { getDrivePerf = origGetter }()

	callCount := 0
	getDrivePerf = func(idx uint32) (diskPerformance, error) {
		callCount++
		baseRead := int64(idx * 1000)
		baseWrite := int64(idx * 2000)
		// Alternate between baseline and collection
		if callCount <= 3 {
			return diskPerformance{BytesRead: baseRead, BytesWritten: baseWrite}, nil
		}
		return diskPerformance{BytesRead: baseRead + 500, BytesWritten: baseWrite + 1000}, nil
	}

	cache := &DriveCache{
		AllowedDrives: map[uint32]MountInfo{
			0: {Model: "Drive0"},
			1: {Model: "Drive1"},
			2: {Model: "Drive2"},
		},
		DriveLetterMap: map[uint32][]string{
			0: {"C:"},
			1: {"D:"},
			2: {"E:", "F:"},
		},
	}

	// Baseline
	_, _ = CollectDiskIO(context.Background(), cache)
	*fakeTime = fakeTime.Add(5 * time.Second)

	// Collection
	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(result))
	}

	// Check device names
	deviceNames := make(map[string]bool)
	for _, m := range result {
		metric := m.(protocol.DiskIOMetric)
		deviceNames[metric.Device] = true
	}

	expectedDevices := []string{"Drive0 (C:)", "Drive1 (D:)", "Drive2 (E:, F:)"}
	for _, expected := range expectedDevices {
		if !deviceNames[expected] {
			t.Errorf("missing expected device %q", expected)
		}
	}
}

func TestCollectDiskIO_DriveError(t *testing.T) {
	lastDiskPerf = nil
	fakeTime := mockTime(t)

	origGetter := getDrivePerf
	defer func() { getDrivePerf = origGetter }()

	callCount := 0
	getDrivePerf = func(idx uint32) (diskPerformance, error) {
		callCount++
		// Drive 1 always fails
		if idx == 1 {
			return diskPerformance{}, errors.New("drive not accessible")
		}
		if callCount <= 2 {
			return diskPerformance{BytesRead: 1000}, nil
		}
		return diskPerformance{BytesRead: 2000}, nil
	}

	cache := &DriveCache{
		AllowedDrives: map[uint32]MountInfo{
			0: {Model: "GoodDrive"},
			1: {Model: "BadDrive"},
		},
		DriveLetterMap: map[uint32][]string{
			0: {"C:"},
			1: {"D:"},
		},
	}

	// Baseline
	_, _ = CollectDiskIO(context.Background(), cache)
	*fakeTime = fakeTime.Add(5 * time.Second)

	// Collection - should still return metrics for working drive
	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 metric (failing drive skipped), got %d", len(result))
	}

	metric := result[0].(protocol.DiskIOMetric)
	if metric.Device != "GoodDrive (C:)" {
		t.Errorf("expected C: to be reported, got %s", metric.Device)
	}
}

func TestCollectDiskIO_NewDriveAppears(t *testing.T) {
	lastDiskPerf = nil
	fakeTime := mockTime(t)

	origGetter := getDrivePerf
	defer func() { getDrivePerf = origGetter }()

	getDrivePerf = func(idx uint32) (diskPerformance, error) {
		return diskPerformance{BytesRead: int64(idx * 1000)}, nil
	}

	cache := &DriveCache{
		AllowedDrives:  map[uint32]MountInfo{0: {Model: "Drive0"}},
		DriveLetterMap: map[uint32][]string{0: {"C:"}},
	}

	// Baseline with one drive
	_, _ = CollectDiskIO(context.Background(), cache)
	*fakeTime = fakeTime.Add(5 * time.Second)

	// Add a new drive
	cache.AllowedDrives[1] = MountInfo{Model: "NewDrive"}
	cache.DriveLetterMap[1] = []string{"D:"}

	// Collection - new drive should be skipped (no baseline)
	result, err := CollectDiskIO(context.Background(), cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 metric (new drive has no baseline), got %d", len(result))
	}

	metric := result[0].(protocol.DiskIOMetric)
	if metric.Device != "Drive0 (C:)" {
		t.Errorf("expected only C: to be reported, got %s", metric.Device)
	}
}

func BenchmarkFormatDeviceName(b *testing.B) {
	di := MountInfo{Model: "Samsung SSD 990 PRO"}
	lm := map[uint32][]string{0: {"C:", "D:"}}

	b.ResetTimer()
	for b.Loop() {
		formatDeviceName(0, di, lm)
	}
}

func BenchmarkCollectDiskIO(b *testing.B) {
	ctx := context.Background()
	cache := &DriveCache{
		AllowedDrives:  map[uint32]MountInfo{0: {Model: "BenchDrive"}},
		DriveLetterMap: map[uint32][]string{0: {"C:"}},
	}

	fakeTime := mockTime(b)

	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectDiskIO(ctx, cache)
		*fakeTime = fakeTime.Add(1 * time.Second)
	}
}
