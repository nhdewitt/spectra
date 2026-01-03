//go:build !windows

package collector

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseMountsFrom(t *testing.T) {
	input := `
rootfs / rootfs rw 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
/dev/sda1 / ext4 rw,relatime,errors=remount-ro 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=813876k,mode=755 0 0
/dev/loop0 /snap/core/123 squashfs ro,nodev,relatime 0 0
/dev/sdb1 /mnt/data xfs rw,relatime 0 0
//192.168.1.5/share /mnt/nfs nfs4 rw,relatime 0 0
C:\ /mnt/wsl/docker-desktop-data ext4 rw,relatime 0 0
`
	reader := strings.NewReader(input)
	mounts, err := parseMountsFrom(reader)
	if err != nil {
		t.Fatalf("parseMountsFrom failed: %v", err)
	}

	// shouldIgnore should remove: rootfs, sysfs, proc, tmpfs, /dev/loop0, /mnt/wsl/docker
	// Should keep: /dev/sda1, /dev/sdb1, //192.168.1.5/share

	expectedCount := 3
	if len(mounts) != expectedCount {
		t.Errorf("Expected %d mounts, got %d", expectedCount, len(mounts))
		for i, m := range mounts {
			t.Logf("[%d] Kept: %s on %s (%s)", i, m.Device, m.Mountpoint, m.FSType)
		}
	}

	foundRoot := false
	for _, m := range mounts {
		if m.Mountpoint == "/" && m.FSType == "ext4" {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Error("Did not find ext4 root mount in output")
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name string
		info MountInfo
		want bool
	}{
		{
			"Standard Ext4",
			MountInfo{
				Device:     "/dev/sda1",
				Mountpoint: "/",
				FSType:     "ext4",
			},
			false,
		},
		{
			"Loop Device",
			MountInfo{
				Device:     "/dev/loop3",
				Mountpoint: "/snap",
				FSType:     "squashfs",
			},
			true,
		},
		{
			"WSL Mount",
			MountInfo{
				Device:     "C:",
				Mountpoint: "/mnt/wsl/docker",
				FSType:     "ext4",
			},
			true,
		},
		{
			"Docker Mount",
			MountInfo{
				Device:     "overlay",
				Mountpoint: "/Docker/stuff",
				FSType:     "overlay",
			},
			true,
		},
		{
			"Ignored FSType",
			MountInfo{
				Device:     "proc",
				Mountpoint: "/proc",
				FSType:     "proc",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIgnore(tt.info)
			if got != tt.want {
				t.Errorf("shouldIgnore(%+v) = %v, want %v", tt.info, got, tt.want)
			}
		})
	}
}

func TestDeviceMapCreation(t *testing.T) {
	mounts := []MountInfo{
		{
			Device:     "/dev/sda1",
			Mountpoint: "/",
		},
		{
			Device:     "/dev/nvme0n1p2",
			Mountpoint: "/home",
		},
	}

	m := createDeviceToMountpointMap(mounts)

	if len(m) != 2 {
		t.Errorf("Expected map len 2, got %d", len(m))
	}

	if _, ok := m["sda1"]; !ok {
		t.Error("Map missing key 'sda1'")
	}
	if _, ok := m["nvme0n1p2"]; !ok {
		t.Error("Map missing key 'nvme0n1p2'")
	}
}

func TestMountManager_Race_Linux(t *testing.T) {
	cache := NewDriveCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go RunMountManager(ctx, cache, 1*time.Millisecond)

	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				cache.RWMutex.RLock()
				_ = len(cache.DeviceToMountpoint)
				cache.RWMutex.RUnlock()
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stopReader)
}
