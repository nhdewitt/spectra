package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	// Limit concurrent requests to prevent choking the Docker daemon
	DockerConcurrencyLimit = 32

	dockerSource  = "docker"
	kindContainer = "container"
)

type DockerClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	Close() error
}

var dockerCli DockerClient

type DockerStats struct {
	CPUStats    DockerCPUStats                `json:"cpu_stats"`
	PreCPUStats DockerCPUStats                `json:"precpu_stats"`
	MemoryStats DockerMemoryStats             `json:"memory_stats"`
	Networks    map[string]DockerNetworkStats `json:"networks"`
}

type DockerCPUStats struct {
	CPUUsage struct {
		TotalUsage  uint64   `json:"total_usage"`
		PercpuUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint32 `json:"online_cpus"`
}

type DockerMemoryStats struct {
	Usage uint64            `json:"usage"`
	Limit uint64            `json:"limit"`
	Stats map[string]uint64 `json:"stats"`
}

type DockerNetworkStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

func InitDocker() error {
	var err error
	dockerCli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	return err
}

func collectDockerContainers(ctx context.Context) ([]protocol.ContainerMetric, error) {
	if dockerCli == nil {
		if err := InitDocker(); err != nil {
			return nil, fmt.Errorf("docker init failed: %w", err)
		}
	}

	// List Containers
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			// Avoid error spamming on agents where Docker isn't installed/running
			return nil, nil
		}

		return nil, fmt.Errorf("docker list failed: %w", err)
	}

	if len(containers) == 0 {
		return []protocol.ContainerMetric{}, nil
	}

	type result struct {
		metric protocol.ContainerMetric
		ok     bool
	}

	results := make(chan result, len(containers))
	sem := make(chan struct{}, DockerConcurrencyLimit)

	for _, c := range containers {
		go func(c container.Summary) {
			sem <- struct{}{}
			defer func() { <-sem }()

			statsReader, err := dockerCli.ContainerStats(ctx, c.ID, false)
			if err != nil {
				results <- result{ok: false}
				return
			}

			var stats DockerStats
			err = json.NewDecoder(statsReader.Body).Decode(&stats)
			statsReader.Body.Close()
			if err != nil {
				results <- result{ok: false}
				return
			}

			cpuPercent := calculateCPUPercent(&stats)

			memUsage := float64(stats.MemoryStats.Usage)
			if v, ok := stats.MemoryStats.Stats["inactive_file"]; ok {
				memUsage -= float64(v)
			}
			if memUsage < 0 {
				memUsage = float64(stats.MemoryStats.Usage)
			}

			numCores := uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
			if stats.CPUStats.OnlineCPUs > 0 {
				numCores = stats.CPUStats.OnlineCPUs
			}

			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}

			rxBytes, txBytes := calculateNet(stats.Networks)

			results <- result{
				metric: protocol.ContainerMetric{
					ID:            id,
					Name:          strings.TrimPrefix(c.Names[0], "/"),
					Image:         c.Image,
					State:         c.State,
					Source:        dockerSource,
					Kind:          kindContainer,
					CPUPercent:    cpuPercent,
					CPULimitCores: numCores,
					MemoryBytes:   uint64(memUsage),
					MemoryLimit:   stats.MemoryStats.Limit,
					NetRxBytes:    rxBytes,
					NetTxBytes:    txBytes,
				},
				ok: true,
			}
		}(c)
	}

	metrics := make([]protocol.ContainerMetric, 0, len(containers))
	for range containers {
		r := <-results
		if r.ok {
			metrics = append(metrics, r.metric)
		}
	}

	return metrics, nil
}

func calculateCPUPercent(v *DockerStats) float64 {
	var cpuPercent float64
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	onlineCPUs := float64(v.CPUStats.OnlineCPUs)

	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	return cpuPercent
}

func calculateNet(networks map[string]DockerNetworkStats) (rxTotal, txTotal uint64) {
	for _, net := range networks {
		rxTotal += net.RxBytes
		txTotal += net.TxBytes
	}
	return rxTotal, txTotal
}
