//go:build freebsd

package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/unix"
)

func CollectTemperature(ctx context.Context) ([]protocol.Metric, error) {
	nCores, err := unix.SysctlUint32("hw.ncpu")
	if err != nil {
		return nil, err
	}

	temps := make([]protocol.Metric, 0, nCores)
	for i := range nCores {
		name := fmt.Sprintf("cpu%d", i)
		tempStr, err := unix.Sysctl(fmt.Sprintf("dev.cpu.%d.temperature", i))
		if err != nil {
			continue
		}
		temp, err := strconv.ParseFloat(strings.TrimSuffix(tempStr, "C"), 64)
		if err != nil {
			continue
		}

		temps = append(temps, protocol.TemperatureMetric{
			Sensor: name,
			Temp:   temp,
			Max:    0.0, // FreeBSD sysctl has no per-core max
		})
	}
	// Check ACPI thermal zones 0-15
	for i := range 16 {
		name := fmt.Sprintf("acpi_tz%d", i)
		tempStr, err := unix.Sysctl(fmt.Sprintf("hw.acpi.thermal.tz%d.temperature", i))
		if err != nil {
			break // no more zones to check
		}
		temp, err := strconv.ParseFloat(strings.TrimSuffix(tempStr, "C"), 64)
		if err != nil || temp < -200 {
			continue
		}

		var max float64
		maxStr, err := unix.Sysctl(fmt.Sprintf("hw.acpi.thermal.tz%d._CRT", i))
		if err == nil {
			max, _ = strconv.ParseFloat(strings.TrimSuffix(maxStr, "C"), 64)
		}

		temps = append(temps, protocol.TemperatureMetric{
			Sensor: name,
			Temp:   temp,
			Max:    max,
		})
	}

	return temps, nil
}
