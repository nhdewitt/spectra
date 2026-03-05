//go:build darwin

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// MakeTemperatureCollector returns a no-op on Darwin.
// macOS thermal sensors require either CGo or a helper
// binary. SMC key names vary by Mac model, making a pure
// Go implementation fragile.
func MakeTemperatureCollector(_ []string) CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return nil, nil
	}
}
