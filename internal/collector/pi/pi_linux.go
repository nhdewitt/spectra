//go:build linux

package pi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CollectClocks gathers Raspberry Pi specific frequency protocol.
// It requires the `vcgencmd` tool (usually pre-installed).
//
// The Pi collectors are registered only when Platform.IsRaspberryPi, so a
// vcgencmd failure here means the agent lost access to the VideoCore mailbox,
// and is worth a log line.
func CollectClocks(ctx context.Context) ([]protocol.Metric, error) {
	// ARM CPU Frequency. sysfs, not vcgencmd. scaling_cur_freq is genuinely
	// absent on some Pi configurations, so a zero here is not an error.
	armFreq := getCPUScalingFreq()

	// VideoCore Frequencies (Core & 3D)
	coreFreq, coreErr := parseFreq(ctx, "core")
	gpuFreq, gpuErr := parseFreq(ctx, "v3d")
	err := errors.Join(coreErr, gpuErr)

	if armFreq == 0 && coreFreq == 0 && gpuFreq == 0 {
		return nil, err
	}

	return []protocol.Metric{
		protocol.ClockMetric{
			ArmFreq:  armFreq,
			CoreFreq: coreFreq,
			GPUFreq:  gpuFreq,
		},
	}, err
}

func CollectVoltage(ctx context.Context) ([]protocol.Metric, error) {
	core, coreErr := parseVolts(ctx, "core")
	sdramC, sdramCErr := parseVolts(ctx, "sdram_c")
	sdramI, sdramIErr := parseVolts(ctx, "sdram_i")
	sdramP, sdramPErr := parseVolts(ctx, "sdram_p")
	err := errors.Join(coreErr, sdramCErr, sdramIErr, sdramPErr)

	if core == 0 && sdramC == 0 {
		return nil, err
	}

	return []protocol.Metric{
		protocol.VoltageMetric{
			Core:   core,
			SDRamC: sdramC,
			SDRamI: sdramI,
			SDRamP: sdramP,
		},
	}, err
}

func CollectThrottle(ctx context.Context) ([]protocol.Metric, error) {
	valStr, err := execVcgencmd(ctx, "get_throttled")
	if err != nil {
		return nil, err
	}

	valStr = strings.TrimPrefix(valStr, "0x")

	val, err := strconv.ParseUint(valStr, 16, 32)
	if err != nil {
		val, err = strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing get_throttled value %q: %w", valStr, err)
		}
	}

	return decodeThrottle(val), nil
}

func decodeThrottle(val uint64) []protocol.Metric {
	// Bitmask Definitions:
	// 0: Undervoltage detected
	// 1: Arm frequency capped
	// 2: Currently throttled
	// 3: Soft temp limit active
	// 16: Undervoltage has occurred
	// 17: Arm frequency capped has occurred
	// 18: Throttling has occurred
	// 19: Soft temp limit has occurred

	return []protocol.Metric{
		protocol.ThrottleMetric{
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

func CollectGPU(ctx context.Context) ([]protocol.Metric, error) {
	totalBytes, err := parseMem(ctx, "gpu")
	if err != nil {
		return nil, err
	}

	return []protocol.Metric{
		protocol.GPUMetric{
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
		return 0, fmt.Errorf("measure_clock %s: %w", block, err)
	}

	freq, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing measure_clock %s value %q: %w", block, valStr, err)
	}
	return freq, nil
}

func parseVolts(ctx context.Context, block string) (float64, error) {
	valStr, err := execVcgencmd(ctx, "measure_volts", block)
	if err != nil {
		return 0, fmt.Errorf("measure_volts %s: %w", block, err)
	}

	valStr = strings.TrimSuffix(valStr, "V")
	volts, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing measure_volts %s value %q: %w", block, valStr, err)
	}
	return volts, nil
}

func parseMem(ctx context.Context, memType string) (uint64, error) {
	valStr, err := execVcgencmd(ctx, "get_mem", memType)
	if err != nil {
		return 0, fmt.Errorf("get_mem %s: %w", memType, err)
	}

	total, err := parseMemString(valStr)
	if err != nil {
		return 0, fmt.Errorf("parsing get_mem %s value %q: %w", memType, valStr, err)
	}
	return total, nil
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
		return "", fmt.Errorf("running vcgencmd %s: %w", strings.Join(args, " "), err)
	}

	return vcgencmdValue(string(out)), nil
}

// vcgencmdValue extracts the value from vcgencmd's "key=value" output
// (e.g. "frequency(0)=500000000" yields "500000000"). Output with no "="
// is returned whole. Values never contain "=", so splitting on the first
// is sufficient.
func vcgencmdValue(out string) string {
	s := strings.TrimSpace(out)
	if _, after, found := strings.Cut(s, "="); found {
		return after
	}
	return s
}
