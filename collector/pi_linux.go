//go:build linux && (arm || arm64)

package collector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/metrics"
)

// CollectPiClocks gathers Raspberry Pi specific frequency metrics.
// It requires the `vcgencmd` tool (usually pre-installed)
func CollectPiClocks(ctx context.Context) ([]metrics.Metric, error) {
	// ARM CPU Frequency
	armFreq := getCPUScalingFreq()

	// VideoCore Frequencies (Core & 3D)
	coreFreq := getVcgencmdFreq(ctx, "core")
	gpuFreq := getVcgencmdFreq(ctx, "v3d")

	if armFreq == 0 && coreFreq == 0 && gpuFreq == 0 {
		return nil, nil
	}

	return []metrics.Metric{
		metrics.ClockMetric{
			ArmFreq:  armFreq,
			CoreFreq: coreFreq,
			GPUFreq:  gpuFreq,
		},
	}, nil
}

// getCPUScalingFreq reads the current CPU frequency from sysfs.
// Returns Hz.
func getCPUScalingFreq() uint64 {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err != nil {
		return 0
	}

	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	return val * 1000
}

// getVcgencmdFreq shells out to `vcgencmd measure_clock <block>`
func getVcgencmdFreq(ctx context.Context, block string) uint64 {
	out, err := exec.CommandContext(ctx, "vcgencmd", "measure_clock", block).Output()
	if err != nil {
		return 0
	}

	// Parse output ("frequency(1)=400000000")
	parts := bytes.Split(out, []byte("="))
	if len(parts) != 2 {
		return 0
	}

	valStr := string(bytes.TrimSpace(parts[1]))
	val, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0
	}

	return val
}

func CollectPiVoltage(ctx context.Context) ([]metrics.Metric, error) {
	core, _ := getVcgencmdVolts(ctx, "core")
	sdramC, _ := getVcgencmdVolts(ctx, "sdram_c")
	sdramI, _ := getVcgencmdVolts(ctx, "sdram_i")
	sdramP, _ := getVcgencmdVolts(ctx, "sdram_p")

	if core == 0 && sdramC == 0 {
		return nil, nil
	}

	return []metrics.Metric{
		metrics.VoltageMetric{
			Core:   core,
			SDRamC: sdramC,
			SDRamI: sdramI,
			SDRamP: sdramP,
		},
	}, nil
}

func getVcgencmdVolts(ctx context.Context, block string) (float64, error) {
	out, err := exec.CommandContext(ctx, "vcgencmd", "measure_volts", block).Output()
	if err != nil {
		return 0, err
	}

	// Parse output ("volt=1.2000V")
	s := strings.TrimSpace(string(out))
	if idx := strings.Index(s, "="); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, "V")

	return strconv.ParseFloat(s, 64)
}

func CollectPiThrottle(ctx context.Context) ([]metrics.Metric, error) {
	out, err := exec.CommandContext(ctx, "vcgencmd", "get_throttled").Output()
	if err != nil {
		return nil, nil
	}

	// Parse output ("throttled=0x50005")
	s := strings.TrimSpace(string(out))
	if idx := strings.Index(s, "="); idx != -1 {
		s = s[idx+1:]
	}

	// Parse Hex
	val, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return nil, err
	}

	// Bitmask Definitions:
	// 0: Undervoltage detected
	// 1: Arm frequency capped
	// 2: Currently throttled
	// 3: Soft temp limit active
	// 16: Undervoltage has occurred
	// 17: Arm frequency capped has occurred
	// 18: Throttling has occurred
	// 19: Soft temp limit has occurred

	return []metrics.Metric{
		metrics.ThrottleMetric{
			// Current Status
			Undervoltage:  (val & (1 << 0)) != 0,
			ArmFreqCapped: (val & (1 << 1)) != 0,
			Throttled:     (val & (1 << 2)) != 0,
			SoftTempLimit: (val & (1 << 3)) != 0,

			// Historical Status
			UndervoltageOccurred:  (val & (1 << 16)) != 0,
			FreqCapOccurred:       (val & (1 << 17)) != 0,
			ThrottledOccurred:     (val & (1 << 18)) != 0,
			SoftTempLimitOccurred: (val & (1 << 19)) != 0,
		},
	}, nil
}

func CollectPiGPU(ctx context.Context) ([]metrics.Metric, error) {
	totalBytes, err := getVcgencmdMem(ctx, "gpu")
	if err != nil {
		return nil, nil
	}

	return []metrics.Metric{
		metrics.GPUMetric{
			MemoryTotal: totalBytes,
			MemoryUsed:  0,
		},
	}, nil
}

func getVcgencmdMem(ctx context.Context, memType string) (uint64, error) {
	out, err := exec.CommandContext(ctx, "vcgencmd", "get_mem", memType).Output()
	if err != nil {
		return 0, err
	}

	// Parse ("gpu=64M")
	s := strings.TrimSpace(string(out))
	parts := strings.Split(s, "=")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid format")
	}

	valStr := parts[1]

	unit := valStr[len(valStr)-1]
	numStr := valStr[:len(valStr)-1]

	val, err := strconv.ParseUint(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	switch unit {
	case 'M':
		return val * 1024 * 1024, nil
	case 'K':
		return val * 1024, nil
	case 'G':
		return val * 1024 * 1024 * 1024, nil
	default:
		return val, nil
	}
}
