//go:build darwin

package temperature

import (
	"context"

	"github.com/nhdewitt/spectra/internal/collector"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// MakeCollector returns a no-op on Darwin.
// macOS thermal sensors require either CGo or a helper
// binary. SMC key names vary by Mac model, making a pure
// Go implementation fragile.
func MakeCollector(_ []string) collector.CollectFunc {
	return func(ctx context.Context) ([]protocol.Metric, error) {
		return nil, nil
	}
}
