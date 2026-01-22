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

	// shouldIgnore should remove: rootfs, sysfs, proc, tmpfs, /dev/loop0, /mnt/wsl/docker, //192.168.1.5/share
	// Should keep: /dev/sda1, /dev/sdb1

	expectedCount := 2
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

func TestParseMountsFrom_Empty(t *testing.T) {
	reader := strings.NewReader("")
	mounts, err := parseMountsFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(mounts))
	}
}

func TestParseMountsFrom_MalformedLines(t *testing.T) {
	input := `
/dev/sda1 / ext4 rw 0 0
short line
/dev/sdb1 /data xfs rw 0 0

just-one-field
/dev/sdc1 /backup btrfs rw 0 0
`
	reader := strings.NewReader(input)
	mounts, err := parseMountsFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 3 {
		t.Errorf("expected 3 mounts, got %d", len(mounts))
	}
}

func TestParseMountsFrom_SpecialMountpoints(t *testing.T) {
	input := `
/dev/sda1 /path/with\040spaces ext4 rw 0 0
/dev/sdb1 /path/with/special#chars xfs rw 0 0
/dev/sdc1 /mnt/usb-drive_0 ext4 rw 0 0
`
	reader := strings.NewReader(input)
	mounts, err := parseMountsFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 3 {
		t.Errorf("expected 3 mounts, got %d", len(mounts))
	}

	if mounts[0].Mountpoint != "/path/with spaces" {
		t.Errorf("expected '/path/with spaces', got %q", mounts[0].Mountpoint)
	}
}

func TestParseMountsFrom_AllFilteredOut(t *testing.T) {
	input := `
proc /proc proc rw 0 0
sysfs /sys sysfs rw 0 0
tmpfs /tmp tmpfs rw 0 0
/dev/loop0 /snap/core squashfs ro 0 0
`
	reader := strings.NewReader(input)
	mounts, err := parseMountsFrom(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 0 {
		t.Errorf("expected all mounts filtered, got %d", len(mounts))
		for _, m := range mounts {
			t.Logf("  kept: %s %s %s", m.Device, m.Mountpoint, m.FSType)
		}
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

func TestShouldIgnore_AllLocalFilesystems(t *testing.T) {
	localFS := []string{"ext4", "ext3", "ext2", "xfs", "btrfs", "vfat", "exfat", "ntfs", "f2fs", "zfs"}

	for _, fs := range localFS {
		info := MountInfo{
			Device:     "/dev/sda1",
			Mountpoint: "/mnt/test",
			FSType:     fs,
		}
		if shouldIgnore(info) {
			t.Errorf("shouldIgnore() = true for local filesystem %q", fs)
		}
	}
}

func TestShouldIgnore_LoopDeviceVariants(t *testing.T) {
	tests := []struct {
		device string
		want   bool
	}{
		{"/dev/loop0", true},
		{"/dev/loop99", true},
		{"/dev/loop123", true},
		{"/dev/loopback", true},
		{"/dev/sda1", false},
		{"/dev/nvme0n1p1", false},
		{"/dev/mmcblk0p1", false},
	}

	for _, tt := range tests {
		info := MountInfo{
			Device:     tt.device,
			Mountpoint: "/mnt/test",
			FSType:     "ext4",
		}
		got := shouldIgnore(info)
		if got != tt.want {
			t.Errorf("shouldIgnore(%s) = %v, want %v", tt.device, got, tt.want)
		}
	}
}

func TestCreateDeviceToMountpointMap(t *testing.T) {
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

func TestCreateDeviceToMountpointMap_Empty(t *testing.T) {
	m := createDeviceToMountpointMap(nil)
	if m == nil {
		t.Error("expected non-nil map")
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestCreateDeviceToMountpointMap_NoDevPrefix(t *testing.T) {
	mounts := []MountInfo{
		{Device: "tmpfs", Mountpoint: "/tmp", FSType: "tmpfs"},
		{Device: "overlay", Mountpoint: "/var/lib/docker", FSType: "overlay"},
	}

	m := createDeviceToMountpointMap(mounts)

	if _, ok := m["tmpfs"]; !ok {
		t.Error("expected 'tmpfs' key")
	}
	if _, ok := m["overlay"]; !ok {
		t.Error("expected 'overlay' key")
	}
}

func TestCreateDeviceToMountpointMap_DuplicateDevices(t *testing.T) {
	mounts := []MountInfo{
		{Device: "/dev/sda1", Mountpoint: "/", FSType: "ext4"},
		{Device: "/dev/sda1", Mountpoint: "/mnt/backup", FSType: "ext4"},
	}

	m := createDeviceToMountpointMap(mounts)

	if len(m) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m))
	}
	if m["sda1"].Mountpoint != "/" {
		t.Errorf("expected first mountpoint to win, got %s", m["sda1"].Mountpoint)
	}
}

func TestMountManager_Race_Linux(t *testing.T) {
	cache := NewDriveCache()
	ctx := t.Context()

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

func TestParseMounts_Integration(t *testing.T) {
	mounts, err := parseMounts()
	if err != nil {
		t.Fatalf("parseMounts failed: %v", err)
	}

	if len(mounts) == 0 {
		t.Error("expected at least one mount")
	}

	foundRoot := false
	for _, m := range mounts {
		if m.Mountpoint == "/" {
			foundRoot = true
			t.Logf("Root: %s (%s)", m.Device, m.FSType)
		}
	}

	if !foundRoot {
		t.Error("root filesystem not found")
	}

	t.Logf("Found %d mounts total", len(mounts))
}

func BenchmarkParseMountsFrom_Minimal(b *testing.B) {
	input := `
/dev/sda1 / ext4 rw,relatime 0 0
/dev/sda2 /home ext4 rw,relatime 0 0
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseMountsFrom(r)
	}
}

func BenchmarkParseMountsFrom_Realistic(b *testing.B) {
	input := `
rootfs / rootfs rw 0 0
sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
udev /dev devtmpfs rw,nosuid,relatime,size=8192k,nr_inodes=2048,mode=755 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=1024k,mode=755 0 0
/dev/sda1 / ext4 rw,relatime,errors=remount-ro 0 0
/dev/sda2 /home ext4 rw,relatime 0 0
/dev/sdb1 /mnt/data xfs rw,relatime 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup2 /sys/fs/cgroup/unified cgroup2 rw,nosuid,nodev,noexec,relatime 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
/dev/loop0 /snap/core/12345 squashfs ro,nodev,relatime 0 0
/dev/loop1 /snap/firefox/1234 squashfs ro,nodev,relatime 0 0
/dev/loop2 /snap/gnome/5678 squashfs ro,nodev,relatime 0 0
`
	b.ReportAllocs()
	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = parseMountsFrom(r)
	}
}

func BenchmarkShouldIgnore_Allowed(b *testing.B) {
	info := MountInfo{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		FSType:     "ext4",
	}
	for b.Loop() {
		_ = shouldIgnore(info)
	}
}

func BenchmarkShouldIgnore_IgnoredFS(b *testing.B) {
	info := MountInfo{
		Device:     "proc",
		Mountpoint: "/proc",
		FSType:     "proc",
	}
	for b.Loop() {
		_ = shouldIgnore(info)
	}
}

func BenchmarkShouldIgnore_LoopDevice(b *testing.B) {
	info := MountInfo{
		Device:     "/dev/loop0",
		Mountpoint: "/snap/core",
		FSType:     "squashfs",
	}
	for b.Loop() {
		_ = shouldIgnore(info)
	}
}

func BenchmarkCreateDeviceToMountpointMap(b *testing.B) {
	mounts := []MountInfo{
		{Device: "/dev/sda1", Mountpoint: "/", FSType: "ext4"},
		{Device: "/dev/sda2", Mountpoint: "/home", FSType: "ext4"},
		{Device: "/dev/sdb1", Mountpoint: "/data", FSType: "xfs"},
		{Device: "/dev/nvme0n1p1", Mountpoint: "/fast", FSType: "ext4"},
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = createDeviceToMountpointMap(mounts)
	}
}

func BenchmarkUpdateCache(b *testing.B) {
	cache := NewDriveCache()
	b.ReportAllocs()
	for b.Loop() {
		updateCache(cache)
	}
}
