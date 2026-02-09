//go:build windows || freebsd

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func collectProxmoxGuests(ctx context.Context) ([]protocol.ContainerMetric, error) {
	return nil, nil
}
