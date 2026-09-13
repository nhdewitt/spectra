//go:build linux

package temperature

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/util"
)

// MakeCollector returns a CollectFunc that reads from the
// provided thermal zone paths, avoiding a filepath.Glob on every cycle.
func MakeCollector(zones []string) collector.CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		type reading struct {
			metric protocol.TemperatureMetric
			zone   string
		}

		readings := make([]reading, 0, len(zones))
		counts := make(map[string]int, len(zones))

		for _, zone := range zones {
			m, err := readThermalZone(zone)
			if err != nil || m == nil {
				continue
			}

			readings = append(readings, reading{metric: *m, zone: zone})
			counts[m.Sensor]++
		}

		// thermal_zoneN/type is not unique. Every ACPI zone on a machine
		// reports "acpitz", so a host with two of them emitted two series
		// under one name, interleaved by whichever happened to be written
		// last. Suffix with the kernel's zone number, but only where the
		// name actually collides, so hosts with unambiguous sensors keep
		// their existing series names and their history stays continuous.
		results := make([]protocol.Metric, 0, len(readings))
		for _, r := range readings {
			if counts[r.metric.Sensor] > 1 {
				r.metric.Sensor += zoneNumber(r.zone)
			}
			results = append(results, r.metric)
		}

		return results, nil
	}
}

// zoneNumber returns the trailing digis of a thermal zone directory, so
// /sys/class/thermal/thermal_zone1 yields "1". Falls back to the whole
// base name if there are no trailing digits, which keeps colliding sensors
// distinct even on a layout that does not number its zones.
func zoneNumber(dir string) string {
	base := filepath.Base(dir)

	i := len(base)
	for i > 0 && base[i-1] >= '0' && base[i-1] <= '9' {
		i--
	}
	if i == len(base) {
		return base
	}
	return base[i:]
}

func readThermalZone(dir string) (*protocol.TemperatureMetric, error) {
	fType, err := os.Open(filepath.Join(dir, "type"))
	if err != nil {
		return nil, err
	}
	defer fType.Close()

	fTemp, err := os.Open(filepath.Join(dir, "temp"))
	if err != nil {
		return nil, err
	}
	defer fTemp.Close()

	// trip_point_0_temp - max temp; if missing, pass nil
	fMax, _ := os.Open(filepath.Join(dir, "trip_point_0_temp"))
	if fMax != nil {
		defer fMax.Close()
	}

	return parseThermalZoneFrom(fType, fTemp, fMax)
}

func parseThermalZoneFrom(typeR, tempR, maxR io.Reader) (*protocol.TemperatureMetric, error) {
	// Sensor Name
	nameData, err := io.ReadAll(typeR)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(string(nameData))

	// Current Temp
	tempVal, err := parseThermalValueFrom(tempR)
	if err != nil {
		return nil, err
	}

	// Temp sanity check: ignore obviously false values
	if tempVal < -40 || tempVal > 150 {
		return nil, nil
	}

	var max *float64
	if maxR != nil {
		if v, err := parseThermalValueFrom(maxR); err == nil {
			max = util.NormalizeMax(tempVal, v)
		}
	}

	return &protocol.TemperatureMetric{
		Sensor: name,
		Temp:   tempVal,
		Max:    max,
	}, nil
}

func parseThermalValueFrom(r io.Reader) (float64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, io.ErrUnexpectedEOF
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	// millidegrees -> degrees
	return val / 1000.0, nil
}
