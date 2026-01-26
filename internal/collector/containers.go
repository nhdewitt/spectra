package collector

import (
	"context"
	"fmt"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func CollectContainers(ctx context.Context) ([]protocol.Metric, error) {
	var result []protocol.ContainerMetric

	dockerContainers, dockerErr := collectDockerContainers(ctx)
	result = append(result, dockerContainers...)

	proxmoxGuests, proxmoxErr := collectProxmoxGuests(ctx)
	result = append(result, proxmoxGuests...)

	if dockerErr != nil && proxmoxErr != nil {
		return nil, fmt.Errorf("docker: %v, proxmox: %v", dockerErr, proxmoxErr)
	}

	return []protocol.Metric{
		protocol.ContainerListMetric{Containers: result},
	}, nil
}
