//go:build darwin

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// CollectWiFi is a no-op on Darwin.
func CollectWiFi(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}
