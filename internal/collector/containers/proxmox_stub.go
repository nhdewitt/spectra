//go:build windows || darwin

package containers

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func collectProxmox(ctx context.Context) ([]protocol.ContainerMetric, error) {
	return nil, nil
}
