package collector

import (
	"log"
	"strconv"
	"time"

	"github.com/nhdewitt/spectra/metrics"
	"golang.org/x/exp/constraints"
)

type Numeric interface {
	constraints.Integer | constraints.Float
}

func percent[T Numeric](part, total T) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(part) / float64(total)) * 100.0
}

func rate(delta uint64, seconds float64) uint64 {
	if seconds <= 0 {
		return 0
	}
	return uint64(float64(delta) / seconds)
}

func singleMetric(m metrics.Metric, err error) ([]metrics.Metric, error) {
	if err != nil || m == nil {
		return nil, err
	}
	return []metrics.Metric{m}, nil
}

// makeUintParser returns a function that parses fields[i] as uint64,
// logging errors with source context and returning 0 on failure.
func makeUintParser(fields []string, source string) func(int) uint64 {
	return func(index int) uint64 {
		v, err := strconv.ParseUint(fields[index], 10, 64)
		if err != nil {
			log.Printf("error parsing %s field[%d] = %q: %v", source, index, fields[index], err)
			return 0
		}
		return v
	}
}

// validateTimeDelta calulates seconds elapsed.
// If valid (>0), it returns the delta.
// If invalid (<=0), it logs a warning and returns 0.
func validateTimeDelta(now, last time.Time, source string) float64 {
	delta := now.Sub(last).Seconds()
	if delta <= 0 {
		log.Printf("Warning [%s]: Invalid time delta (%f sec). Clock skew? Now: %v, Last: %v", source, delta, now, last)
		return 0
	}

	return delta
}
