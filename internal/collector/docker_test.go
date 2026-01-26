package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nhdewitt/spectra/internal/protocol"
)

type mockDockerClient struct {
	containers []container.Summary
	statsDelay time.Duration
}

func (m *mockDockerClient) ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error) {
	return m.containers, nil
}

func (m *mockDockerClient) ContainerStats(ctx context.Context, id string, stream bool) (container.StatsResponseReader, error) {
	time.Sleep(m.statsDelay)

	stats := DockerStats{
		CPUStats: DockerCPUStats{
			CPUUsage: struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			}{TotalUsage: 200000000},
			SystemUsage: 1000000000,
			OnlineCPUs:  4,
		},
		PreCPUStats: DockerCPUStats{
			CPUUsage: struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			}{TotalUsage: 100000000},
			SystemUsage: 900000000,
		},
		MemoryStats: DockerMemoryStats{
			Usage: 100000000,
			Limit: 2000000000,
			Stats: map[string]uint64{"inactive_file": 10000},
		},
		Networks: map[string]DockerNetworkStats{
			"eth0": {RxBytes: 1000, TxBytes: 2000},
		},
	}

	data, _ := json.Marshal(stats)
	return container.StatsResponseReader{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

func makeMockContainers(n int) []container.Summary {
	containers := make([]container.Summary, n)
	for i := range containers {
		containers[i] = container.Summary{
			ID:    fmt.Sprintf("abc123def456%048d", i),
			Names: []string{fmt.Sprintf("/container-%d", i)},
			Image: "test:latest",
			State: "running",
		}
	}

	return containers
}

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name  string
		stats DockerStats
		want  float64
	}{
		{
			name: "Normal Usage",
			stats: DockerStats{
				CPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 200000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  4,
				},
				PreCPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 100000000},
					SystemUsage: 900000000,
				},
			},
			want: 400.0, // (100M / 100M) * 4 * 100
		},
		{
			name: "Zero System Delta",
			stats: DockerStats{
				CPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 200000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  4,
				},
				PreCPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 100000000},
					SystemUsage: 1000000000, // Same as current
				},
			},
			want: 0.0,
		},
		{
			name: "Fallback to PercpuUsage Length",
			stats: DockerStats{
				CPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{
						TotalUsage:  200000000,
						PercpuUsage: []uint64{1, 2, 3, 4}, // 4 CPUs
					},
					SystemUsage: 1000000000,
					OnlineCPUs:  0, // Not set
				},
				PreCPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 100000000},
					SystemUsage: 900000000,
				},
			},
			want: 400.0,
		},
		{
			name: "Single CPU",
			stats: DockerStats{
				CPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 150000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  1,
				},
				PreCPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 100000000},
					SystemUsage: 900000000,
				},
			},
			want: 50.0, // (50M / 100M) * 1 * 100
		},
		{
			name: "High CPU Multi-Core",
			stats: DockerStats{
				CPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 800000000},
					SystemUsage: 1000000000,
					OnlineCPUs:  8,
				},
				PreCPUStats: DockerCPUStats{
					CPUUsage: struct {
						TotalUsage  uint64   `json:"total_usage"`
						PercpuUsage []uint64 `json:"percpu_usage"`
					}{TotalUsage: 0},
					SystemUsage: 0,
				},
			},
			want: 640.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercent(&tt.stats)
			if got != tt.want {
				t.Errorf("calculateCPUPercent() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCalculateNet(t *testing.T) {
	tests := []struct {
		name     string
		networks map[string]DockerNetworkStats
		wantRx   uint64
		wantTx   uint64
	}{
		{
			name:     "Empty Networks",
			networks: map[string]DockerNetworkStats{},
			wantRx:   0,
			wantTx:   0,
		},
		{
			name: "Single Network",
			networks: map[string]DockerNetworkStats{
				"eth0": {RxBytes: 1000, TxBytes: 2000},
			},
			wantRx: 1000,
			wantTx: 2000,
		},
		{
			name: "Multiple Networks",
			networks: map[string]DockerNetworkStats{
				"eth0":   {RxBytes: 1000, TxBytes: 2000},
				"eth1":   {RxBytes: 500, TxBytes: 750},
				"bridge": {RxBytes: 100, TxBytes: 200},
			},
			wantRx: 1600,
			wantTx: 2950,
		},
		{
			name:     "Nil Networks",
			networks: nil,
			wantRx:   0,
			wantTx:   0,
		},
		{
			name: "Large Values",
			networks: map[string]DockerNetworkStats{
				"eth0": {RxBytes: 10000000000, TxBytes: 20000000000},
			},
			wantRx: 10000000000,
			wantTx: 20000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRx, gotTx := calculateNet(tt.networks)
			if gotRx != tt.wantRx {
				t.Errorf("calculateNet() rxTotal = %d, want %d", gotRx, tt.wantRx)
			}
			if gotTx != tt.wantTx {
				t.Errorf("calculateNet() txTotal = %d, want %d", gotTx, tt.wantTx)
			}
		})
	}
}

func TestInitDocker(t *testing.T) {
	// Reset global client
	oldCli := dockerCli
	dockerCli = nil
	defer func() { dockerCli = oldCli }()

	err := InitDocker()
	if err != nil {
		t.Logf("InitDocker returned error: %v", err)
		return
	}

	if dockerCli == nil {
		t.Error("dockerCli should not be nil after successful init")
	}
}

func TestCollectDocker_Integration(t *testing.T) {
	oldCli := dockerCli
	dockerCli = nil
	defer func() { dockerCli = oldCli }()

	ctx := context.Background()
	metrics, err := CollectDocker(ctx)
	if err != nil {
		t.Logf("CollectDocker returned error: %v", err)
		return
	}
	if metrics == nil {
		t.Log("No metrics returned (Docker may not exist or may not be running)")
		return
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric container, got %d", len(metrics))
	}

	listMetric, ok := metrics[0].(protocol.ContainerListMetric)
	if !ok {
		t.Fatalf("expected ContainerListMetric, got %T", metrics[0])
	}

	t.Logf("Found %d containers", len(listMetric.Containers))

	for _, c := range listMetric.Containers {
		t.Logf("Container: %s (%s)", c.Name, c.ID)
		t.Logf("  Image: %s", c.Image)
		t.Logf("  State: %s", c.State)
		t.Logf("  CPU: %.2f%% (%d cores)", c.CPUPercent, c.NumProcs)
		t.Logf("  Memory: %d / %d bytes", c.MemoryBytes, c.MemoryLimit)
		t.Logf("  Network: RX=%d TX=%d bytes", c.NetRxBytes, c.NetTxBytes)

		if len(c.ID) != 12 {
			t.Errorf("Container ID should be 12 chars, got %d", len(c.ID))
		}
		if c.Name == "" {
			t.Error("Container name should not be empty")
		}
		if c.CPUPercent < 0 {
			t.Errorf("CPU percent should not be negative: %f", c.CPUPercent)
		}
		if c.MemoryLimit > 0 && c.MemoryBytes > c.MemoryLimit {
			t.Errorf("Memory usage %d exceeds limit %d", c.MemoryBytes, c.MemoryLimit)
		}
	}
}

func TestCollectDocker_NoDocker(t *testing.T) {
	// Test behavior when Docker is not available
	oldCli := dockerCli
	dockerCli = nil
	defer func() { dockerCli = oldCli }()

	// Create a client that will fail to connect
	badCli, _ := client.NewClientWithOpts(client.WithHost("tcp://localhost:99999"))
	dockerCli = badCli

	ctx := context.Background()
	metrics, err := CollectDocker(ctx)
	// Should return nil, nil for connection failures (not spam errors)
	if err != nil {
		t.Logf("CollectDocker error: %v", err)
	}
	t.Logf("Returned %v metrics", metrics)
}

func TestCollectDocker_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectDocker(ctx)
	if err != nil {
		t.Logf("CollectDocker with cancelled context: %v", err)
	}
}

func TestContainerNameTrimming(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/mycontainer", "mycontainer"},
		{"/my-container-name", "my-container-name"},
		{"noprefix", "noprefix"},
		{"/", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := strings.TrimPrefix(tt.input, "/")
		if got != tt.want {
			t.Errorf("TrimPrefix(%q, '/') = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCollectDocker_Parallel(t *testing.T) {
	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := CollectDocker(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent CollectDocker error: %v", err)
	}
}

func TestContainerIDTruncation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Normal ID",
			input: "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			want:  "abc123def456",
		},
		{
			name:  "Exactly 12",
			input: "abc123def456",
			want:  "abc123def456",
		},
		{
			name:  "Short ID",
			input: "abc123",
			want:  "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.input
			if len(id) > 12 {
				id = id[:12]
			}
			if id != tt.want {
				t.Errorf("ID truncation = %q, want %q", id, tt.want)
			}
		})
	}
}

func BenchmarkCalculateCPUPercent(b *testing.B) {
	stats := &DockerStats{
		CPUStats: DockerCPUStats{
			CPUUsage: struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			}{TotalUsage: 200000000},
			SystemUsage: 1000000000,
			OnlineCPUs:  4,
		},
		PreCPUStats: DockerCPUStats{
			CPUUsage: struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			}{TotalUsage: 100000000},
			SystemUsage: 900000000,
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = calculateCPUPercent(stats)
	}
}

func BenchmarkCalculateNet(b *testing.B) {
	networks := map[string]DockerNetworkStats{
		"eth0":   {RxBytes: 1000000, TxBytes: 2000000},
		"eth1":   {RxBytes: 500000, TxBytes: 750000},
		"bridge": {RxBytes: 100000, TxBytes: 200000},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = calculateNet(networks)
	}
}

func BenchmarkCollectDocker(b *testing.B) {
	// Init once
	if dockerCli == nil {
		if err := InitDocker(); err != nil {
			b.Skip("Docker init failed")
		}
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = CollectDocker(ctx)
	}
}

func BenchmarkCollectDocker_Mock25_1sLatency(b *testing.B) {
	dockerCli = &mockDockerClient{
		containers: makeMockContainers(25),
		statsDelay: 1 * time.Second,
	}
	defer func() { dockerCli = nil }()

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, _ = CollectDocker(ctx)
	}
}

func BenchmarkCollectDocker_Mock100_1sLatency(b *testing.B) {
	dockerCli = &mockDockerClient{
		containers: makeMockContainers(100),
		statsDelay: 1 * time.Second,
	}
	defer func() { dockerCli = nil }()

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, _ = CollectDocker(ctx)
	}
}

func BenchmarkCollectDocker_Mock200_1sLatency(b *testing.B) {
	dockerCli = &mockDockerClient{
		containers: makeMockContainers(200),
		statsDelay: 1 * time.Second,
	}
	defer func() { dockerCli = nil }()

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, _ = CollectDocker(ctx)
	}
}

func BenchmarkCollectDocker_Mock500_1sLatency(b *testing.B) {
	dockerCli = &mockDockerClient{
		containers: makeMockContainers(500),
		statsDelay: 1 * time.Second,
	}
	defer func() { dockerCli = nil }()

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, _ = CollectDocker(ctx)
	}
}
