//go:build !windows

package collector

import (
	"context"
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

func TestCollectPi_Integration(t *testing.T) {
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
