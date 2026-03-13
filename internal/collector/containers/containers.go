package containers

import (
	"context"
	"fmt"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func Collect(ctx context.Context) ([]protocol.Metric, error) {
	var result []protocol.ContainerMetric

	dockerContainers, dockerErr := collectDocker(ctx)
	result = append(result, dockerContainers...)

	proxmoxGuests, proxmoxErr := collectProxmox(ctx)
	result = append(result, proxmoxGuests...)

	if dockerErr != nil && proxmoxErr != nil {
		return nil, fmt.Errorf("docker: %w, proxmox: %w", dockerErr, proxmoxErr)
	}

	return []protocol.Metric{
		protocol.ContainerListMetric{Containers: result},
	}, nil
}
