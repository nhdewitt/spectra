//go:build darwin

package disk

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

func setupMountCache(t *testing.T) *DriveCache {
	t.Helper()
	cache := &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	mounts, err := parseMounts()
	if err != nil {
		t.Fatalf("failed to parse mounts: %v", err)
	}

	newMap := createDeviceToMountpointMap(mounts)
	cache.DeviceToMountpoint = newMap

	return cache
}

func TestFsCategory(t *testing.T) {
	tests := []struct {
		fsType, want string
	}{
		{"apfs", "local"},
		{"hfs", "local"},
		{"hfsplus", "local"},
		{"exfat", "local"},
		{"msdos", "local"},
		{"ntfs", "local"},
		{"devfs", "other"},
		{"autofs", "other"},
		{"nfs", "other"},
		{"smbfs", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.fsType, func(t *testing.T) {
			got := fsCategory(tt.fsType)
			if got != tt.want {
				t.Errorf("fsCategory(%q) = %q, want %q", tt.fsType, got, tt.want)
			}
		})
	}
}

func TestBuildDiskMetric(t *testing.T) {
	tests := []struct {
		name string
		info MountInfo
		stat unix.Statfs_t
		want protocol.DiskMetric
	}{
		{
			name: "typical APFS root",
			info: MountInfo{
				Device:     "/dev/disk3s1s1",
				Mountpoint: "/",
				FSType:     "apfs",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 26214400,
				Bfree:  13107200,
				Bavail: 11796480,
				Files:  6553600,
				Ffree:  6000000,
			},
			want: protocol.DiskMetric{
				Device:      "/dev/disk3s1s1",
				Mountpoint:  "/",
				Filesystem:  "apfs",
				Type:        "local",
				Total:       107374182400,
				Used:        53687091200,
				Available:   48318382080,
				UsedPct:     50.0,
				InodesTotal: 6553600,
				InodesUsed:  553600,
				InodesPct:   8.45,
			},
		},
		{
			name: "nearly full disk",
			info: MountInfo{
				Device:     "/dev/disk3s5",
				Mountpoint: "/System/Volumes/Data",
				FSType:     "apfs",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 7864320,
				Bfree:  262144,
				Bavail: 131072,
				Files:  1966080,
				Ffree:  100000,
			},
			want: protocol.DiskMetric{
				Device:      "/dev/disk3s5",
				Mountpoint:  "/System/Volumes/Data",
				Filesystem:  "apfs",
				Type:        "local",
				Total:       32212254720,
				Used:        31138512896,
				Available:   536870912,
				UsedPct:     96.67,
				InodesTotal: 1966080,
				InodesUsed:  1866080,
				InodesPct:   94.91,
			},
		},
		{
			name: "empty disk",
			info: MountInfo{
				Device:     "/dev/disk4s1",
				Mountpoint: "/Volumes/External",
				FSType:     "exfat",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 52428800,
				Bfree:  52428800,
				Bavail: 52428800,
				Files:  1000000,
				Ffree:  1000000,
			},
			want: protocol.DiskMetric{
				Device:      "/dev/disk4s1",
				Mountpoint:  "/Volumes/External",
				Filesystem:  "exfat",
				Type:        "local",
				Total:       214748364800,
				Used:        0,
				Available:   214748364800,
				UsedPct:     0.0,
				InodesTotal: 1000000,
				InodesUsed:  0,
				InodesPct:   0.0,
			},
		},
		{
			name: "other filesystem",
			info: MountInfo{
				Device:     "192.168.1.100:/share",
				Mountpoint: "/Volumes/nfs_share",
				FSType:     "nfs",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 262144000,
				Bfree:  131072000,
				Bavail: 131072000,
				Files:  10000000,
				Ffree:  9000000,
			},
			want: protocol.DiskMetric{
				Device:      "192.168.1.100:/share",
				Mountpoint:  "/Volumes/nfs_share",
				Filesystem:  "nfs",
				Type:        "other",
				Total:       1073741824000,
				Used:        536870912000,
				Available:   536870912000,
				UsedPct:     50.0,
				InodesTotal: 10000000,
				InodesUsed:  1000000,
				InodesPct:   10.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDiskMetric(tt.info, tt.stat)

			if got.Device != tt.want.Device {
				t.Errorf("Device = %q, want %q", got.Device, tt.want.Device)
			}
			if got.Mountpoint != tt.want.Mountpoint {
				t.Errorf("Mountpoint = %q, want %q", got.Mountpoint, tt.want.Mountpoint)
			}
			if got.Filesystem != tt.want.Filesystem {
				t.Errorf("Filesystem = %q, want %q", got.Filesystem, tt.want.Filesystem)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Total != tt.want.Total {
				t.Errorf("Type = %d, want %d", got.Total, tt.want.Total)
			}
			if got.Used != tt.want.Used {
				t.Errorf("Used = %d, want %d", got.Used, tt.want.Used)
			}
			if got.Available != tt.want.Available {
				t.Errorf("Available = %d, want %d", got.Available, tt.want.Available)
			}
			if !approxEqual(got.UsedPct, tt.want.UsedPct, 0.01) {
				t.Errorf("UsedPct = %.2f, want %.2f", got.UsedPct, tt.want.UsedPct)
			}
			if got.InodesTotal != tt.want.InodesTotal {
				t.Errorf("InodesTotal = %d, want %d", got.InodesTotal, tt.want.InodesTotal)
			}
			if got.InodesUsed != tt.want.InodesUsed {
				t.Errorf("InodesUsed = %d, want %d", got.InodesUsed, tt.want.InodesUsed)
			}
			if !approxEqual(got.InodesPct, tt.want.InodesPct, 0.01) {
				t.Errorf("InodesPct = %.2f, want %.2f", got.InodesPct, tt.want.InodesPct)
			}
		})
	}
}

func approxEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= epsilon
}

func TestCollectDisk_EmptyCache(t *testing.T) {
	ctx := context.Background()
	cache := &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	metrics, err := CollectDisk(ctx, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for empty cache, got %d", len(metrics))
	}
}

func TestBuildDiskMetric_ZeroSize(t *testing.T) {
	info := MountInfo{
		Device:     "/dev/disk5s1",
		Mountpoint: "/Volumes/Empty",
		FSType:     "apfs",
	}

	stat := unix.Statfs_t{
		Bsize:  4096,
		Blocks: 0,
		Bfree:  0,
		Bavail: 0,
		Files:  0,
		Ffree:  0,
	}

	got := buildDiskMetric(info, stat)

	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
	if got.UsedPct != 0 {
		t.Errorf("UsedPct = %f, want 0", got.UsedPct)
	}
	if got.InodesPct != 0 {
		t.Errorf("InodesPct = %f, want 0", got.InodesPct)
	}
}

func TestBuildDiskMetric_LargeDisk(t *testing.T) {
	info := MountInfo{
		Device:     "/dev/disk4s1",
		Mountpoint: "/Volumes/Data",
		FSType:     "apfs",
	}

	stat := unix.Statfs_t{
		Bsize:  4096,
		Blocks: 2147483648,
		Bfree:  1073741824,
		Bavail: 1073741824,
		Files:  500000000,
		Ffree:  450000000,
	}

	got := buildDiskMetric(info, stat)

	expectedTotal := uint64(8796093022208)
	if got.Total != expectedTotal {
		t.Errorf("Total = %d, want %d", got.Total, expectedTotal)
	}
	if !approxEqual(got.UsedPct, 50.0, 0.01) {
		t.Errorf("UsedPct = %.2f, want 50.0", got.UsedPct)
	}
}

func TestBuildDiskMetric_ListMounts(t *testing.T) {
	cache := &DriveCache{
		DeviceToMountpoint: map[string]MountInfo{
			"disk3s1s1": {Device: "/dev/disk3s1s1", Mountpoint: "/", FSType: "apfs"},
			"disk3s5":   {Device: "/dev/disk3s5", Mountpoint: "/System/Volumes/Data", FSType: "apfs"},
		},
	}

	mounts := cache.ListMounts()

	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(mounts))
	}

	found := make(map[string]bool)
	for _, m := range mounts {
		found[m.Mountpoint] = true
	}

	if !found["/"] || !found["/System/Volumes/Data"] {
		t.Errorf("missing expected mountpoints: %v", mounts)
	}
}

func TestDriveCache_ListMounts_Empty(t *testing.T) {
	cache := &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	mounts := cache.ListMounts()

	if mounts == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(mounts))
	}
}

func TestParseMounts(t *testing.T) {
	mounts, err := parseMounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mounts) == 0 {
		t.Fatal("expected at least one mount")
	}

	var foundRoot bool
	for _, m := range mounts {
		if m.Mountpoint == "/" {
			foundRoot = true
			if _, local := localFilesystems[m.FSType]; !local {
				t.Errorf("root fstype = %q, expected a local filesystem", m.FSType)
			}
		}

		if _, ignored := ignoredMounts[m.Mountpoint]; ignored {
			t.Errorf("ignored mount %q should have been filtered", m.Mountpoint)
		}
		if _, ignored := ignoredFilesystems[m.FSType]; ignored {
			t.Errorf("ignored filesystem %q should have been filtered", m.FSType)
		}
	}

	if !foundRoot {
		t.Error("root mount not found")
	}

	t.Logf("found %d mounts", len(mounts))
	for _, m := range mounts {
		t.Logf("  %s on %s (%s)", m.Device, m.Mountpoint, m.FSType)
	}
}

func TestCollectDisk_Integration(t *testing.T) {
	ctx := context.Background()
	cache := setupMountCache(t)

	metrics, err := CollectDisk(ctx, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected at least one disk metric")
	}

	for _, m := range metrics {
		dm := m.(protocol.DiskMetric)
		t.Logf("%s on %s (%s): total=%d used=%d avail=%d pct=%.1f%%",
			dm.Device, dm.Mountpoint, dm.Filesystem, dm.Total, dm.Used, dm.Available, dm.UsedPct)

		if dm.Total == 0 && dm.Mountpoint == "/" {
			t.Error("root filesystem reports 0 total bytes")
		}
		if dm.UsedPct < 0 || dm.UsedPct > 100 {
			t.Errorf("%s: UsedPct = %.2f, want 0-100", dm.Mountpoint, dm.UsedPct)
		}
	}
}

func BenchmarkCollectDisk(b *testing.B) {
	ctx := context.Background()
	cache := &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	mounts, err := parseMounts()
	if err != nil {
		b.Fatalf("failed to parse mounts: %v", err)
	}
	cache.DeviceToMountpoint = createDeviceToMountpointMap(mounts)

	diskCollector := MakeDiskCollector(cache)
	b.ResetTimer()

	for b.Loop() {
		diskCollector(ctx)
	}
}

func BenchmarkBuildDiskMetric(b *testing.B) {
	info := MountInfo{
		Device:     "/dev/disk3s1s1",
		Mountpoint: "/",
		FSType:     "apfs",
	}
	stat := unix.Statfs_t{
		Bsize:  4096,
		Blocks: 26214400,
		Bfree:  13107200,
		Bavail: 11796480,
		Files:  6553600,
		Ffree:  6000000,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = buildDiskMetric(info, stat)
	}
}
