//go:build !windows
// +build !windows

package collector

import (
	"context"
	"testing"

	"github.com/nhdewitt/spectra/metrics"
	"golang.org/x/sys/unix"
)

func setupMountCache(b *testing.B) *DriveCache {
	cache := &DriveCache{
		DeviceToMountpoint: make(map[string]MountInfo),
	}

	mounts, err := parseMounts()
	if err != nil {
		b.Fatalf("failed to parse mounts for benchmark setup: %v", err)
	}

	newMap := createDeviceToMountpointMap(mounts)
	cache.DeviceToMountpoint = newMap

	return cache
}

func BenchmarkCollectDisk(b *testing.B) {
	ctx := context.Background()
	mountCache := setupMountCache(b)

	diskCollector := MakeDiskCollector(mountCache)
	b.ResetTimer()

	for b.Loop() {
		diskCollector(ctx)
	}
}

func TestFsCategory(t *testing.T) {
	tests := []struct {
		fsType string
		want   string
	}{
		{"ext4", "local"},
		{"ext3", "local"},
		{"xfs", "local"},
		{"btrfs", "local"},
		{"vfat", "local"},
		{"ntfs", "local"},
		{"nfs", "other"},
		{"cifs", "other"},
		{"tmpfs", "other"},
		{"overlay", "other"},
		{"squashfs", "other"},
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
		want metrics.DiskMetric
	}{
		{
			name: "typical ext4 root partition",
			info: MountInfo{
				Device:     "/dev/sda1",
				Mountpoint: "/",
				FSType:     "ext4",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 26214400,
				Bfree:  13107200,
				Bavail: 11796480,
				Files:  6553600,
				Ffree:  6000000,
			},
			want: metrics.DiskMetric{
				Device:      "/dev/sda1",
				Mountpoint:  "/",
				Filesystem:  "ext4",
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
				Device:     "/dev/mmcblk0p1",
				Mountpoint: "/",
				FSType:     "ext4",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 7864320,
				Bfree:  262144,
				Bavail: 131072,
				Files:  1966080,
				Ffree:  100000,
			},
			want: metrics.DiskMetric{
				Device:      "/dev/mmcblk0p1",
				Mountpoint:  "/",
				Filesystem:  "ext4",
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
				Device:     "/dev/sdb1",
				Mountpoint: "/mnt/data",
				FSType:     "xfs",
			},
			stat: unix.Statfs_t{
				Bsize:  4096,
				Blocks: 52428800,
				Bfree:  52428800,
				Bavail: 52428800,
				Files:  1000000,
				Ffree:  1000000,
			},
			want: metrics.DiskMetric{
				Device:      "/dev/sdb1",
				Mountpoint:  "/mnt/data",
				Filesystem:  "xfs",
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
			name: "small boot partition vfat",
			info: MountInfo{
				Device:     "/dev/mmcblk0p1",
				Mountpoint: "/boot",
				FSType:     "vfat",
			},
			stat: unix.Statfs_t{
				Bsize:  512,
				Blocks: 524288,
				Bfree:  419430,
				Bavail: 419430,
				Files:  0,
				Ffree:  0,
			},
			want: metrics.DiskMetric{
				Device:      "/dev/mmcblk0p1",
				Mountpoint:  "/boot",
				Filesystem:  "vfat",
				Type:        "local",
				Total:       268435456,
				Used:        53687296,
				Available:   214748160,
				UsedPct:     20.00,
				InodesTotal: 0,
				InodesUsed:  0,
				InodesPct:   0.0,
			},
		},
		{
			name: "non-local filesystem",
			info: MountInfo{
				Device:     "192.168.1.100:/share",
				Mountpoint: "/mnt/nfs",
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
			want: metrics.DiskMetric{
				Device:      "192.168.1.100:/share",
				Mountpoint:  "/mnt/nfs",
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
			t.Logf("Input: Bsize=%d Blocks=%d Bavail=%d", tt.stat.Bsize, tt.stat.Blocks, tt.stat.Bavail)
			got := buildDiskMetric(tt.info, tt.stat)
			t.Logf("Output: Total=%d Used=%d Available=%d", got.Total, got.Used, got.Available)

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
				t.Errorf("Total = %d, want %d", got.Total, tt.want.Total)
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
