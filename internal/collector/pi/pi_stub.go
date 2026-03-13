//go:build windows || freebsd || darwin

package pi

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CollectClocks is a no-op on Windows
func CollectClocks(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}

// CollectVoltage is a no-op on Windows
func CollectVoltage(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}

// CollectThrottle is a no-op on Windows
func CollectThrottle(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}

// CollectGPU is a no-op on Windows
func CollectGPU(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}
