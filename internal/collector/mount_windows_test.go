//go:build windows

package collector

import (
	"sync"
	"testing"
)

func TestNewDriveCache_Windows(t *testing.T) {
	cache := NewDriveCache()

	if cache == nil {
		t.Fatal("NewDriveCache returned nil")
	}
	if cache.AllowedDrives == nil {
		t.Fatal("AllowedDrives map is nil")
	}
	if cache.DriveLetterMap == nil {
		t.Error("DriveLetterMap is nil")
	}
	if len(cache.AllowedDrives) != 0 {
		t.Errorf("expected empty AllowedDrives, got %d entries", len(cache.AllowedDrives))
	}
	if len(cache.DriveLetterMap) != 0 {
		t.Errorf("expected empty DriveLetterMap, got %d entries", len(cache.DriveLetterMap))
	}
}

func TestDriveCache_GetDefaultPath_WithC(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"C:"}
	cache.DriveLetterMap[1] = []string{"D:", "E:"}

	got := cache.GetDefaultPath()
	if got != "C:\\" {
		t.Errorf("expected 'C:\\', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_COnSecondDrive(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"D:"}
	cache.DriveLetterMap[1] = []string{"C:", "E:"}

	got := cache.GetDefaultPath()
	if got != "C:\\" {
		t.Errorf("expected 'C:\\', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_NoC(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"D:"}
	cache.DriveLetterMap[1] = []string{"E:", "F:"}

	got := cache.GetDefaultPath()
	if got != "D:\\" && got != "E:\\" {
		t.Errorf("expected 'D:\\' or 'E:\\', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_Empty(t *testing.T) {
	cache := NewDriveCache()

	got := cache.GetDefaultPath()
	if got != "." {
		t.Errorf("expected '.', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_EmptyLetterSlice(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{}
	cache.DriveLetterMap[1] = []string{"D:"}

	got := cache.GetDefaultPath()
	if got != "D:\\" {
		t.Errorf("expected 'D:\\', got %q", got)
	}
}

func TestDriveCache_GetDefaultPath_AllEmptySlices(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{}
	cache.DriveLetterMap[1] = []string{}

	got := cache.GetDefaultPath()
	if got != "." {
		t.Errorf("expected '.', got %q", got)
	}
}

func TestDriveCache_Concurrency_Windows(t *testing.T) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"C:"}
	cache.AllowedDrives[0] = MountInfo{
		DeviceID: `\\.\PHYSICALDRIVE0`,
		Index:    0,
		Model:    "Test Drive",
	}

	var wg sync.WaitGroup

	// Concurrent readers
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				path := cache.GetDefaultPath()
				if path != "C:\\" {
					t.Errorf("unexpected path: %q", path)
				}
			}
		}()
	}

	// Concurrent writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			cache.Lock()
			cache.DriveLetterMap[1] = []string{"D:"}
			cache.AllowedDrives[1] = MountInfo{
				DeviceID: `\\.\PHYSICALDRIVE1`,
				Index:    1,
				Model:    "Second Drive",
			}
			cache.Unlock()
		}
	}()

	wg.Wait()
}

func BenchmarkDriveCache_GetDefaultPath_WithC(b *testing.B) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"C:"}
	cache.DriveLetterMap[1] = []string{"D:", "E:"}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_COnSecondDrive(b *testing.B) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"D:"}
	cache.DriveLetterMap[1] = []string{"C:", "E:"}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_NoC(b *testing.B) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"D:"}
	cache.DriveLetterMap[1] = []string{"E:", "F:"}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_Empty_Windows(b *testing.B) {
	cache := NewDriveCache()

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}

func BenchmarkDriveCache_GetDefaultPath_Contended_Windows(b *testing.B) {
	cache := NewDriveCache()
	cache.DriveLetterMap[0] = []string{"C:"}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cache.GetDefaultPath()
		}
	})
}

func BenchmarkDriveCache_GetDefaultPath_ManyDrives(b *testing.B) {
	cache := NewDriveCache()

	for i := range 10 {
		letter := string(rune('D'+i)) + ":"
		cache.DriveLetterMap[uint32(i)] = []string{letter}
	}
	cache.DriveLetterMap[10] = []string{"C:"}

	b.ResetTimer()
	for b.Loop() {
		_ = cache.GetDefaultPath()
	}
}
