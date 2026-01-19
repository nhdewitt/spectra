//go:build windows

package collector

import (
	"strings"
	"testing"
	"time"
)

func TestGetPlatformInfo(t *testing.T) {
	platform, version := getPlatformInfo()

	if platform == "" {
		t.Error("platform should not be empty")
	}

	if !strings.Contains(strings.ToLower(platform), "windows") {
		t.Errorf("platform should contain 'Windows', got %q", platform)
	}

	t.Logf("Platform: %s, Version: %s", platform, version)
}

func TestGetKernel(t *testing.T) {
	build := getKernel()

	if build == "" {
		t.Error("kernel build should not be empty")
	}
	if len(build) < 4 {
		t.Errorf("unexpected build format: %q", build)
	}

	t.Logf("Kernel build: %s", build)
}

func TestGetCPUModel(t *testing.T) {
	cpu := getCPUModel()

	if cpu == "" {
		t.Error("CPU model should not be empty")
	}

	t.Logf("CPU Model: %s", cpu)
}

func TestGetRAMTotal(t *testing.T) {
	ram := getRAMTotal()

	if ram == 0 {
		t.Error("RAM total should not be zero")
	}

	minRAM := uint64(256 * 1024 * 1024)
	maxRAM := uint64(64 * 1024 * 1024 * 1024 * 1024)

	if ram < minRAM {
		t.Errorf("RAM seems too low: %d bytes", ram)
	}
	if ram > maxRAM {
		t.Errorf("RAM seems too high: %d bytes", ram)
	}

	t.Logf("Total RAM: %d bytes (%.2f GB)", ram, float64(ram)/(1024*1024*1024))
}

func TestGetBootTime(t *testing.T) {
	bootTime := getBootTime()
	if bootTime == 0 {
		t.Error("boot time should not be zero")
	}

	now := time.Now().Unix()
	if bootTime > now {
		t.Errorf("boot time %d is in the future (now: %d)", bootTime, now)
	}

	minTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if bootTime < minTime {
		t.Errorf("boot time %d is before year 2000", bootTime)
	}

	uptime := now - bootTime
	t.Logf("Boot time: %d (uptime: %d seconds / %.1f hours)", bootTime, uptime, float64(uptime)/3600)
}

func TestGetBootTime_Consistency(t *testing.T) {
	boot1 := getBootTime()
	time.Sleep(10 * time.Millisecond)
	boot2 := getBootTime()

	diff := boot2 - boot1
	if diff < -1 {
		t.Errorf("second boot time (%d) drifted behind first boot time call (%d)", boot2, boot1)
	}

	if diff > 1 {
		t.Errorf("boot time inconsistent: %d vs %d (diff: %d)", boot1, boot2, diff)
	}
}

func BenchmarkGetPlatformInfo(b *testing.B) {
	for b.Loop() {
		getPlatformInfo()
	}
}

func BenchmarkGetKernel(b *testing.B) {
	for b.Loop() {
		getKernel()
	}
}

func BenchmarkGetCPUModel(b *testing.B) {
	for b.Loop() {
		getCPUModel()
	}
}

func BenchmarkGetRAMTotal(b *testing.B) {
	for b.Loop() {
		getRAMTotal()
	}
}

func BenchmarkGetBootTime(b *testing.B) {
	for b.Loop() {
		getBootTime()
	}
}
