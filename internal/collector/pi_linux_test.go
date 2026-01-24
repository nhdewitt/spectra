//go:build !windows

package collector

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestParseMemString(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"256M", 268435456},
		{"1G", 1073741824},
		{"1024K", 1048576},
		{"512", 512},
		{"", 0},
	}

	for _, tt := range tests {
		got, err := parseMemString(tt.input)
		if err != nil {
			t.Errorf("parseMemString(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseMemString(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseMemString_EdgeCases(t *testing.T) {
	tests := []struct {
		input   string
		want    uint64
		wantErr bool
	}{
		{"0M", 0, false},
		{"0", 0, false},
		{"4096M", 4294967296, false},
		{"16G", 17179869184, false},
		{"invalid", 0, true},
		{"M", 0, true},
		{"123X", 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMemString(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("parseMemString(%q) expected error, got nil", tt.input)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("parseMemString(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseMemString(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeThrottle(t *testing.T) {
	tests := []struct {
		val   uint64
		check func(protocol.ThrottleMetric) bool
		name  string
	}{
		{
			val:  65537,
			name: "Undervoltage Now + History",
			check: func(m protocol.ThrottleMetric) bool {
				return m.Undervoltage == true && m.UndervoltageOccurred == true
			},
		},
		{
			val:  4,
			name: "Throttled Now Only",
			check: func(m protocol.ThrottleMetric) bool {
				return m.Throttled == true && m.ThrottledOccurred == false
			},
		},
		{
			val:  0,
			name: "All Good",
			check: func(m protocol.ThrottleMetric) bool {
				return !m.Throttled && !m.Undervoltage && !m.SoftTempLimit
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeThrottle(tt.val)
			if len(result) == 0 {
				t.Fatal("decodeThrottle returned no metrics")
			}
			m := result[0].(protocol.ThrottleMetric)
			if !tt.check(m) {
				t.Errorf("Check failed for val %d", tt.val)
			}
		})
	}
}

func TestDecodeThrottle_AllBits(t *testing.T) {
	tests := []struct {
		name  string
		val   uint64
		field string
		want  bool
	}{
		{"Undervoltage bit 0", 1 << 0, "Undervoltage", true},
		{"ArmFreqCapped bit 1", 1 << 1, "ArmFreqCapped", true},
		{"Throttled bit 2", 1 << 2, "Throttled", true},
		{"SoftTempLimit bit 3", 1 << 3, "SoftTempLimit", true},
		{"UndervoltageOccurred bit 16", 1 << 16, "UndervoltageOccurred", true},
		{"FreqCapOccurred bit 17", 1 << 17, "FreqCapOccurred", true},
		{"ThrottledOccurred bit 18", 1 << 18, "ThrottledOccurred", true},
		{"SoftTempLimitOccurred bit 19", 1 << 19, "SoftTempLimitOccurred", true},
		{"All current bits", 0xF, "AllCurrent", true},
		{"All history bits", 0xF0000, "AllHistory", true},
		{"All bits set", 0xF000F, "AllSet", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeThrottle(tt.val)
			m := result[0].(protocol.ThrottleMetric)

			switch tt.field {
			case "Undervoltage":
				if m.Undervoltage != tt.want {
					t.Errorf("Undervoltage = %v, want %v", m.Undervoltage, tt.want)
				}
			case "ArmFreqCapped":
				if m.ArmFreqCapped != tt.want {
					t.Errorf("ArmFreqCapped = %v, want %v", m.ArmFreqCapped, tt.want)
				}
			case "Throttled":
				if m.Throttled != tt.want {
					t.Errorf("Throttled = %v, want %v", m.Throttled, tt.want)
				}
			case "SoftTempLimit":
				if m.SoftTempLimit != tt.want {
					t.Errorf("SoftTempLimit = %v, want %v", m.SoftTempLimit, tt.want)
				}
			case "UndervoltageOccurred":
				if m.UndervoltageOccurred != tt.want {
					t.Errorf("UndervoltageOccurred = %v, want %v", m.UndervoltageOccurred, tt.want)
				}
			case "FreqCapOccurred":
				if m.FreqCapOccurred != tt.want {
					t.Errorf("FreqCapOccurred = %v, want %v", m.FreqCapOccurred, tt.want)
				}
			case "ThrottledOccurred":
				if m.ThrottledOccurred != tt.want {
					t.Errorf("ThrottledOccurred = %v, want %v", m.ThrottledOccurred, tt.want)
				}
			case "SoftTempLimitOccurred":
				if m.SoftTempLimitOccurred != tt.want {
					t.Errorf("SoftTempLimitOccurred = %v, want %v", m.SoftTempLimitOccurred, tt.want)
				}
			case "AllCurrent":
				if !m.Undervoltage || !m.ArmFreqCapped || !m.Throttled || !m.SoftTempLimit {
					t.Error("Not all current flags set")
				}
			case "AllHistory":
				if !m.UndervoltageOccurred || !m.FreqCapOccurred || !m.ThrottledOccurred || !m.SoftTempLimitOccurred {
					t.Error("Not all history flags set")
				}
			case "AllSet":
				// All should be true
			}
		})
	}
}

func TestGetCPUScalingFreq(t *testing.T) {
	freq := getCPUScalingFreq()

	if _, err := os.Stat("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); os.IsNotExist(err) {
		if freq != 0 {
			t.Errorf("expected 0 when sysfs not available, got %d", freq)
		}
		t.Skip("cpufreq sysfs not available")
	}

	if freq == 0 {
		t.Error("expected non-zero frequency")
	}

	if freq < 100_000_000 || freq > 10_000_000_000 {
		t.Errorf("frequency %d Hz seems unreasonable", freq)
	}

	t.Logf("CPU scaling freq: %d Hz (%.2F GHz)", freq, float64(freq)/1e9)
}

func TestCollectPiClocks_NoVcgencmd(t *testing.T) {
	if _, err := exec.LookPath("vcgencmd"); err == nil {
		t.Skip("vcgencmd is available, skipping test")
	}

	ctx := context.Background()
	result, err := CollectPiClocks(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	t.Logf("Result without vcgencmd: %v", result)
}

func TestCollectPiVoltage_NoVcgencmd(t *testing.T) {
	if _, err := exec.LookPath("vcgencmd"); err == nil {
		t.Skip("vcgencmd is available, skipping test")
	}

	ctx := context.Background()
	result, err := CollectPiClocks(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil without vcgencmd, got %v", result)
	}
}

func TestCollectPiClocks_Integration(t *testing.T) {
	path, err := exec.LookPath("vcgencmd")
	if err != nil {
		t.Skip("vcgencmd not found - skipping hardware integration test")
	}
	t.Logf("Found vcgencmd at: %s", path)

	ctx := context.Background()

	clocks, err := CollectPiClocks(ctx)
	if err != nil {
		t.Errorf("CollectPiClocks failed: %v", err)
	}
	if len(clocks) > 0 {
		c := clocks[0].(protocol.ClockMetric)
		t.Logf("ARM: %d Hz, Core: %d Hz", c.ArmFreq, c.CoreFreq)
		if c.CoreFreq == 0 {
			t.Error("Core frequency reported as 0 Hz")
		}
	}

	volts, err := CollectPiVoltage(ctx)
	if err != nil {
		t.Errorf("CollectPiVoltage failed: %v", err)
	}
	if len(volts) > 0 {
		v := volts[0].(protocol.VoltageMetric)
		t.Logf("Core Voltage: %.4f V", v.Core)
		if v.Core < 0.1 {
			t.Errorf("Core voltage reported as below 0.1 V")
		}
	}

	_, err = CollectPiThrottle(ctx)
	if err != nil {
		t.Errorf("CollectPiThrottle failed: %v", err)
	}

	gpu, err := CollectPiGPU(ctx)
	if err != nil {
		t.Errorf("CollectPiGPU failed: %v", err)
	}
	if len(gpu) > 0 {
		g := gpu[0].(protocol.GPUMetric)
		t.Logf("GPU Mem: %d bytes", g.MemoryTotal)
		if g.MemoryTotal == 0 {
			t.Error("GPU Memory reported as 0")
		}
	}
}

func BenchmarkDecodeThrottle(b *testing.B) {
	for b.Loop() {
		_ = decodeThrottle(0x50005)
	}
}

func BenchmarkParseMemString(b *testing.B) {
	for b.Loop() {
		_, _ = parseMemString("256M")
	}
}

func BenchmarkGetCPUScalingFreq(b *testing.B) {
	if _, err := os.Stat("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); os.IsNotExist(err) {
		b.Skip("cpufreq sysfs not available")
	}

	for b.Loop() {
		_ = getCPUScalingFreq()
	}
}

func BenchmarkCollectPiClocks(b *testing.B) {
	if _, err := exec.LookPath("vcgencmd"); err != nil {
		b.Skip("vcgencmd not available")
	}

	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectPiClocks(ctx)
	}
}

func BenchmarkCollectPiVoltage(b *testing.B) {
	if _, err := exec.LookPath("vcgencmd"); err != nil {
		b.Skip("vcgencmd not available")
	}

	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectPiVoltage(ctx)
	}
}

func BenchmarkCollectPiThrottle(b *testing.B) {
	if _, err := exec.LookPath("vcgencmd"); err != nil {
		b.Skip("vcgencmd not available")
	}

	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectPiThrottle(ctx)
	}
}

func BenchmarkCollectPiGPU(b *testing.B) {
	if _, err := exec.LookPath("vcgencmd"); err != nil {
		b.Skip("vcgencmd not available")
	}

	ctx := context.Background()
	for b.Loop() {
		_, _ = CollectPiGPU(ctx)
	}
}
