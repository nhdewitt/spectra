//go:build windows

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/metrics"
)

// CollectPiClocks is a no-op on Windows
func CollectPiClocks(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiClocks is a no-op on Windows
func CollectPiVoltage(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiClocks is a no-op on Windows
func CollectPiThrottle(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiClocks is a no-op on Windows
func CollectPiGPU(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}
