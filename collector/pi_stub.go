//go:build !linux || (!arm && !arm64)

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/metrics"
)

// CollectPiClocks is a no-op on non-ARM Linux systems
func CollectPiClocks(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiVotage is a no-op on non-ARM Linux systems
func CollectPiVoltage(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiThrottle is a no-op on non-ARM Linux systems
func CollectPiThrottle(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}

// CollectPiGPU is a no-op on non-ARM Linux systems
func CollectPiGPU(ctx context.Context) ([]metrics.Metric, error) {
	return nil, nil
}
