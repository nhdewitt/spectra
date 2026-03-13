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
		var results []protocol.Metric
		for _, zone := range zones {
			if m, err := readThermalZone(zone); err == nil {
				results = append(results, *m)
			}
		}
		return results, nil
	}
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
