//go:build windows

package collector

import (
	"context"
	"strings"

	"github.com/nhdewitt/spectra/metrics"
	"github.com/yusufpapurcu/wmi"
)

// MSAcpi_ThermalZoneTemperature maps to the WMI class.
// The tags tell the library which WMI properties to load.
type MSAcpi_ThermalZoneTemperature struct {
	CurrentTemperature uint32
	CriticalTripPoint  uint32
	InstanceName       string
}

func CollectTemperature(ctx context.Context) ([]metrics.Metric, error) {
	var dst []MSAcpi_ThermalZoneTemperature

	q := wmi.CreateQuery(&dst, "")

	err := wmi.QueryNamespace(q, &dst, `root\wmi`)
	if err != nil {
		return nil, nil
	}

	var results []metrics.Metric
	for _, v := range dst {
		// Calculate Temp: Decikelvin -> Celsius
		celsius := (float64(v.CurrentTemperature) - 2732.0) / 10.0

		// Calc Max Temp: CriticalTripPoint
		maxCelsius := 0.0
		if v.CriticalTripPoint > 0 {
			maxCelsius = (float64(v.CriticalTripPoint) - 2732.0) / 10.0
		}

		// Clean Name
		name := v.InstanceName
		if lastIdx := strings.LastIndex(name, `\`); lastIdx != -1 {
			name = name[lastIdx+1:]
		}

		results = append(results, metrics.TemperatureMetric{
			Sensor: name,
			Temp:   celsius,
			Max:    maxCelsius,
		})
	}

	return results, nil
}
