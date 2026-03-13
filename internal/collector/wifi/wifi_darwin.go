//go:build darwin

package wifi

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// Collect is a no-op on Darwin.
func Collect(ctx context.Context) ([]protocol.Metric, error) {
	return nil, nil
}
