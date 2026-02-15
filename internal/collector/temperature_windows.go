//go:build windows

package collector

import (
	"context"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/yusufpapurcu/wmi"
)

// MSAcpi_ThermalZoneTemperature maps to the WMI class.
// The tags tell the library which WMI properties to load.
type MSAcpi_ThermalZoneTemperature struct {
	CurrentTemperature uint32
	CriticalTripPoint  uint32
	InstanceName       string
}

// MakeTemperatureCollector returns CollectTemperature unchanged on Windows.
// Thermal zones are detected via WMI, not sysfs paths.
func MakeTemperatureCollector(_ []string) CollectFunc {
	return CollectTemperature
}

func CollectTemperature(ctx context.Context) ([]protocol.Metric, error) {
	var dst []MSAcpi_ThermalZoneTemperature

	q := wmi.CreateQuery(&dst, "")

	err := wmi.QueryNamespace(q, &dst, `root\wmi`)
	if err != nil {
		return nil, nil
	}

	var results []protocol.Metric
	for _, v := range dst {
		// Calculate Temp: Decikelvin -> Celsius
		celsius := (float64(v.CurrentTemperature) - 2732.0) / 10.0

		// Calc Max Temp: CriticalTripPoint
		var max *float64
		if v.CriticalTripPoint > 0 {
			raw := (float64(v.CriticalTripPoint) - 2732.0) / 10.0
			max = normalizeMax(celsius, raw)
		}

		// Clean Name
		name := v.InstanceName
		if lastIdx := strings.LastIndex(name, `\`); lastIdx != -1 {
			name = name[lastIdx+1:]
		}

		results = append(results, protocol.TemperatureMetric{
			Sensor: name,
			Temp:   celsius,
			Max:    max,
		})
	}

	return results, nil
}
