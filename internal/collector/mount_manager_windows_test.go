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

func TestDriveCacheIntegration(t *testing.T) {
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

func TestMountManager_Race(t *testing.T) {
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
