//go:build !windows

package collector

import (
	"context"
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
	coreFreq, _ := parseFreq(ctx, "core")
	gpuFreq, _ := parseFreq(ctx, "v3d")

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

func CollectPiVoltage(ctx context.Context) ([]metrics.Metric, error) {
	core, _ := parseVolts(ctx, "core")
	sdramC, _ := parseVolts(ctx, "sdram_c")
	sdramI, _ := parseVolts(ctx, "sdram_i")
	sdramP, _ := parseVolts(ctx, "sdram_p")

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

func CollectPiThrottle(ctx context.Context) ([]metrics.Metric, error) {
	valStr, err := execVcgencmd(ctx, "get_throttled")
	if err != nil {
		return nil, nil
	}

	valStr = strings.TrimPrefix(valStr, "0x")

	val, err := strconv.ParseUint(valStr, 16, 32)
	if err != nil {
		val, err = strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			return nil, err
		}
	}

	return decodeThrottle(val), nil
}

func decodeThrottle(val uint64) []metrics.Metric {
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
			Undervoltage:          (val & (1 << 0)) != 0,
			ArmFreqCapped:         (val & (1 << 1)) != 0,
			Throttled:             (val & (1 << 2)) != 0,
			SoftTempLimit:         (val & (1 << 3)) != 0,
			UndervoltageOccurred:  (val & (1 << 16)) != 0,
			FreqCapOccurred:       (val & (1 << 17)) != 0,
			ThrottledOccurred:     (val & (1 << 18)) != 0,
			SoftTempLimitOccurred: (val & (1 << 19)) != 0,
		},
	}
}

func CollectPiGPU(ctx context.Context) ([]metrics.Metric, error) {
	totalBytes, err := parseMem(ctx, "gpu")
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

func parseFreq(ctx context.Context, block string) (uint64, error) {
	valStr, err := execVcgencmd(ctx, "measure_clock", block)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(valStr, 10, 64)
}

func parseVolts(ctx context.Context, block string) (float64, error) {
	valStr, err := execVcgencmd(ctx, "measure_volts", block)
	if err != nil {
		return 0, err
	}
	valStr = strings.TrimSuffix(valStr, "V")
	return strconv.ParseFloat(valStr, 64)
}

func parseMem(ctx context.Context, memType string) (uint64, error) {
	valStr, err := execVcgencmd(ctx, "get_mem", memType)
	if err != nil {
		return 0, err
	}
	return parseMemString(valStr)
}

func parseMemString(valStr string) (uint64, error) {
	if len(valStr) == 0 {
		return 0, nil
	}

	unit := valStr[len(valStr)-1]
	if unit >= '0' && unit <= '9' {
		return strconv.ParseUint(valStr, 10, 64)
	}

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

// execVcgencmd runs the command and returns the value part of "key=value"
func execVcgencmd(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "vcgencmd", args...).Output()
	if err != nil {
		return "", err
	}

	s := strings.TrimSpace(string(out))
	if idx := strings.Index(s, "="); idx != -1 {
		return s[idx+1:], nil
	}
	return s, nil
}
