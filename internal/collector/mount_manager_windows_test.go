//go:build windows

package collector

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExtractString(t *testing.T) {
	// Mock buffer simulating raw C-string data
	buf := make([]byte, 20)
	copy(buf[5:], []byte("TestModel"))
	buf[14] = 0 // NULL terminator

	tests := []struct {
		name   string
		buf    []byte
		offset uint32
		want   string
	}{
		{"Normal String", buf, 5, "TestModel"},
		{"Out of Bounds", buf, 50, ""},
		{"Empty Buffer", []byte{}, 0, ""},
		{"Zero Offset", []byte{'A', 'B', 0}, 0, "AB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractString(tt.buf, tt.offset)
			if got != tt.want {
				t.Errorf("extractString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractString_NoNullTerminator(t *testing.T) {
	buf := []byte("NoTerminator")
	got := extractString(buf, 0)
	if got != "NoTerminator" {
		t.Errorf("expected 'NoTerminator', got %q", got)
	}
}

func TestExtractString_AllNulls(t *testing.T) {
	buf := make([]byte, 10)
	got := extractString(buf, 0)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractString_NullAtOffset(t *testing.T) {
	buf := []byte("Hello\x00World")
	got := extractString(buf, 6)
	if got != "World" {
		t.Errorf("expected 'World', got %q", got)
	}
}

func TestNewDriveCache(t *testing.T) {
	cache := NewDriveCache()

	if cache == nil {
		t.Fatal("NewDriveCache returned nil")
	}
	if cache.AllowedDrives == nil {
		t.Error("AllowedDrives is nil")
	}
	if cache.DriveLetterMap == nil {
		t.Error("DriveLetterMap is nil")
	}
}

func TestNewDriveCache_Integration(t *testing.T) {
	// Create a fresh cache
	cache := NewDriveCache()

	// Update
	updateDriveCacheNative(cache)

	// Verify Locking & Data
	cache.RWMutex.RLock()
	defer cache.RWMutex.RUnlock()

	t.Logf("Found %d physical drives", len(cache.AllowedDrives))
	t.Logf("Found %d drive mappings", len(cache.DriveLetterMap))

	// Validate physical drives
	if len(cache.AllowedDrives) == 0 {
		t.Log("WARNING: No physical drives found.")
	}

	foundCDrive := false
	for idx, drive := range cache.AllowedDrives {
		t.Logf("PhysicalDrive%d: Model=%q Interface=%d", idx, drive.Model, drive.InterfaceType)

		if drive.DeviceID == "" {
			t.Error("Drive has empty DeviceID")
		}

		if letters, ok := cache.DriveLetterMap[idx]; ok {
			t.Logf("  -> Mapped to: %v", letters)
			for _, l := range letters {
				if strings.EqualFold(l, "C:") {
					foundCDrive = true
				}
			}
		}
	}

	// Validate C: mapping
	if len(cache.AllowedDrives) > 0 && !foundCDrive {
		t.Error("Could not find a physical drive mapped to C:.")
	}
}

func TestRunMountManager_Race(t *testing.T) {
	cache := NewDriveCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go RunMountManager(ctx, cache, 10*time.Millisecond)

	// Simulate concurrent readers
	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				cache.RWMutex.RLock()
				_ = len(cache.AllowedDrives)
				cache.RWMutex.RUnlock()
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stopReader)
}

func TestRunMountManager_ContextCancel(t *testing.T) {
	cache := NewDriveCache()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		RunMountManager(ctx, cache, 1*time.Hour)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RunMountManager did not exit on context cancel")
	}
}

func TestScanPhysicalDrives_Integration(t *testing.T) {
	drives := scanPhysicalDrives()

	if len(drives) == 0 {
		t.Error("expected at least one physical drive")
	}

	for i, d := range drives {
		t.Logf("Drive %d: Index=%d Model=%q Interface=%d", i, d.Index, d.Model, d.InterfaceType)

		if d.DeviceID == "" {
			t.Errorf("drive %d has empty DeviceID", i)
		}
		if d.Model == "" {
			t.Errorf("drive %d has empty Model", i)
		}
	}
}

func TestMapDriveLettersToPhysicalDisks_Integration(t *testing.T) {
	drives := scanPhysicalDrives()

	allowedMap := make(map[uint32]MountInfo)
	for _, d := range drives {
		allowedMap[d.Index] = d
	}

	letterMap := mapDriveLettersToPhysicalDisks(allowedMap)

	if len(letterMap) == 0 {
		t.Error("expected at least one drive letter mapping")
	}

	foundC := false
	for idx, letters := range letterMap {
		t.Logf("PhysicalDrive%d -> %v", idx, letters)
		for _, l := range letters {
			if strings.EqualFold(l, "C:") {
				foundC = true
			}
		}
	}

	if !foundC {
		t.Error("C: drive not found in mappings")
	}
}

func TestGetPhysicalDiskNumber_CDrive(t *testing.T) {
	diskNum, err := getPhysicalDiskNumber("C:")
	if err != nil {
		t.Fatalf("failed to get disk number of C: - %v", err)
	}

	t.Logf("C: is on PhysicalDrive%d", diskNum)
}

func TestGetPhysicalDiskNumber_InvalidDrive(t *testing.T) {
	// Q: is unlikely to exist
	_, err := getPhysicalDiskNumber("Z:")
	if err != nil {
		t.Logf("Q: returned error: %v", err)
	}
}

func TestUpdateDriveCacheNative_FiltersUSB(t *testing.T) {
	cache := NewDriveCache()
	updateDriveCacheNative(cache)

	cache.RLock()
	defer cache.RUnlock()

	for idx, drive := range cache.AllowedDrives {
		switch drive.InterfaceType {
		case BusTypeUsb:
			t.Errorf("PhysicalDrive%d: USB drive should be filtered", idx)
		case BusType1394:
			t.Errorf("PhysicalDrive%d: 1394 drive should be filtered", idx)
		}
		if strings.Contains(strings.ToLower(drive.Model), "virtual") {
			t.Errorf("PhysicalDrive%d: Virtual drive should be filtered", idx)
		}
	}
}

func TestMakeDiskCollector(t *testing.T) {
	cache := NewDriveCache()
	collector := MakeDiskCollector(cache)

	if collector == nil {
		t.Fatal("MakeDiskCollector returned nil")
	}

	metrics, err := collector(context.Background())
	if err != nil {
		t.Fatalf("collector failed: %v", err)
	}

	t.Logf("DiskCollector returned %d metrics", len(metrics))
}

func TestMakeDiskIOCollector(t *testing.T) {
	cache := NewDriveCache()
	updateDriveCacheNative(cache)
	collector := MakeDiskIOCollector(cache)

	if collector == nil {
		t.Fatal("MakeDiskIOCollector returned nil")
	}

	// Baseline
	_, err := collector(context.Background())
	if err != nil {
		t.Fatalf("collector failed: %v", err)
	}
}

func BenchmarkExtractString(b *testing.B) {
	buf := []byte("TestString\x00Padding")
	b.ResetTimer()
	for b.Loop() {
		extractString(buf, 0)
	}
}

func BenchmarkUpdateDriveCache(b *testing.B) {
	cache := NewDriveCache()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		updateDriveCacheNative(cache)
	}
}

func BenchmarkScanPhysicalDrives(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = scanPhysicalDrives()
	}
}

func BenchmarkMapDriveLettersToPhysicalDisks(b *testing.B) {
	drives := scanPhysicalDrives()
	allowedMap := make(map[uint32]MountInfo)
	for _, d := range drives {
		allowedMap[d.Index] = d
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = mapDriveLettersToPhysicalDisks(allowedMap)
	}
}

func BenchmarkGetPhysicalDiskNumber(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = getPhysicalDiskNumber("C:")
	}
}
