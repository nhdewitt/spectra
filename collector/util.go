package collector

import (
	"github.com/nhdewitt/raspimon/metrics"
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

func singleMetric(m metrics.Metric, err error) ([]metrics.Metric, error) {
	if err != nil || m == nil {
		return nil, err
	}
	return []metrics.Metric{m}, nil
}
