//go:build windows

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/metrics"
)

func CollectSystem(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}
