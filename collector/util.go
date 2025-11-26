package collector

import "github.com/nhdewitt/raspimon/metrics"

func percent(part, total uint64) float64 {
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
