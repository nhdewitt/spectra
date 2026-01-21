//go:build !windows

package collector

import (
	"sync"
	"testing"
)

func TestNewDriveCache(t *testing.T) {
	cache := NewDriveCache()

	if cache == nil {
		t.Fatalf("NewDriveCache returned nil")
	}
	if cache.DeviceToMountpoint == nil {
		t.Error("DeviceToMountpoint map is nil")
	}
	if len(cache.DeviceToMountpoint) != 0 {
		t.Errorf("expected empty map, got %d entries", len(cache.DeviceToMountpoint))
	}
}

func TestDriveCache_GetDefaultPath_WithRoot(t *testing.T) {
	cache := NewDriveCache()
	cache.DeviceToMountpoint["/"] = MountInfo{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		FSType:     "ext4",
	}
	cache.DeviceToMountpoint["/dev/sdb1"] = MountInfo{
		Device:     "/dev/sdb1",
		Mountpoint: "/home",
		FSType:     "ext4",
	}

	got := cache.GetDefaultPath()
	if got != "/" {
		t.Errorf("expected '/', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_NoRoot(t *testing.T) {
	cache := NewDriveCache()
	cache.DeviceToMountpoint["/dev/sdb1"] = MountInfo{
		Device:     "/dev/sdb1",
		Mountpoint: "/data",
		FSType:     "ext4",
	}

	got := cache.GetDefaultPath()
	if got != "/data" {
		t.Errorf("expected '/data', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_Empty(t *testing.T) {
	cache := NewDriveCache()

	got := cache.GetDefaultPath()
	if got != "." {
		t.Errorf("expected '.', got %q", got)
	}
}

func TestDriveCache_Concurrency(t *testing.T) {
	cache := NewDriveCache()

	cache.DeviceToMountpoint["/"] = MountInfo{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		FSType:     "ext4",
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent readers
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				path := cache.GetDefaultPath()
				if path != "/" {
					errors <- nil
				}
			}
		}()
	}

	// Concurrent writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			cache.Lock()
			cache.DeviceToMountpoint["/dev/sdb1"] = MountInfo{
				Device:     "/dev/sdb1",
				Mountpoint: "/data",
				FSType:     "xfs",
			}
			cache.Unlock()
		}
	}()

	wg.Wait()
	close(errors)
}

func BenchmarkDriveCache_GetDefaultPath_WithRoot(b *testing.B) {
	cache := NewDriveCache()
	cache.DeviceToMountpoint["/"] = MountInfo{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		FSType:     "ext4",
	}
	cache.DeviceToMountpoint["/dev/sdb1"] = MountInfo{
		Device:     "/dev/sdb1",
		Mountpoint: "/home",
		FSType:     "ext4",
	}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_NoRoot(b *testing.B) {
	cache := NewDriveCache()
	cache.DeviceToMountpoint["/dev/sdb1"] = MountInfo{
		Device:     "/dev/sdb1",
		Mountpoint: "/data",
		FSType:     "ext4",
	}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_Empty(b *testing.B) {
	cache := NewDriveCache()

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_Contended(b *testing.B) {
	cache := NewDriveCache()
	cache.DeviceToMountpoint["/"] = MountInfo{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		FSType:     "ext4",
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cache.GetDefaultPath()
		}
	})
}
