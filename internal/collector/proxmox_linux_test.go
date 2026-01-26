//go:build !windows

package collector

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestMapProxmoxStatus(t *testing.T) {
	tests := []struct {
		name     string
		resource proxmoxResource
		kind     string
		want     protocol.ContainerMetric
	}{
		{
			name: "LXC Container Running",
			resource: proxmoxResource{
				VMID:   100,
				Name:   "web-server",
				Status: "running",
				CPU:    0.25,
				CPUs:   4,
				Mem:    1073741824,
				MaxMem: 2147483648,
				NetIn:  1000000,
				NetOut: 2000000,
			},
			kind: kindLXC,
			want: protocol.ContainerMetric{
				ID:            "100",
				Name:          "web-server",
				State:         "running",
				Source:        "proxmox",
				Kind:          "lxc",
				CPUPercent:    100.0, // 0.25 * 4 * 100
				CPULimitCores: 4,
				MemoryBytes:   1073741824,
				MemoryLimit:   2147483648,
				NetRxBytes:    1000000,
				NetTxBytes:    2000000,
			},
		},
		{
			name: "VM Stopped",
			resource: proxmoxResource{
				VMID:   200,
				Name:   "database",
				Status: "stopped",
				CPU:    0,
				CPUs:   8,
				Mem:    0,
				MaxMem: 8589934592,
				NetIn:  0,
				NetOut: 0,
			},
			kind: kindVM,
			want: protocol.ContainerMetric{
				ID:            "200",
				Name:          "database",
				State:         "stopped",
				Source:        "proxmox",
				Kind:          "vm",
				CPUPercent:    0.0,
				CPULimitCores: 8,
				MemoryBytes:   0,
				MemoryLimit:   8589934592,
				NetRxBytes:    0,
				NetTxBytes:    0,
			},
		},
		{
			name: "Zero CPUs",
			resource: proxmoxResource{
				VMID:   101,
				Name:   "minimal",
				Status: "running",
				CPU:    0.5,
				CPUs:   0,
				Mem:    512000000,
				MaxMem: 1024000000,
			},
			kind: kindLXC,
			want: protocol.ContainerMetric{
				ID:            "101",
				Name:          "minimal",
				State:         "running",
				Source:        "proxmox",
				Kind:          "lxc",
				CPUPercent:    0.0, // 0 CPUs means 0%
				CPULimitCores: 0,
				MemoryBytes:   512000000,
				MemoryLimit:   1024000000,
				NetRxBytes:    0,
				NetTxBytes:    0,
			},
		},
		{
			name: "High CPU Usage",
			resource: proxmoxResource{
				VMID:   102,
				Name:   "compute",
				Status: "running",
				CPU:    0.95,
				CPUs:   16,
				Mem:    32000000000,
				MaxMem: 64000000000,
				NetIn:  5000000000,
				NetOut: 3000000000,
			},
			kind: kindVM,
			want: protocol.ContainerMetric{
				ID:            "102",
				Name:          "compute",
				State:         "running",
				Source:        "proxmox",
				Kind:          "vm",
				CPUPercent:    1520.0, // 0.95 * 16 * 100
				CPULimitCores: 16,
				MemoryBytes:   32000000000,
				MemoryLimit:   64000000000,
				NetRxBytes:    5000000000,
				NetTxBytes:    3000000000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapProxmoxStatus(tt.resource, tt.kind)

			if got.ID != tt.want.ID {
				t.Errorf("ID = %s, want %s", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %s, want %s", got.Name, tt.want.Name)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %s, want %s", got.State, tt.want.State)
			}
			if got.Source != tt.want.Source {
				t.Errorf("Source = %s, want %s", got.Source, tt.want.Source)
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("Kind = %s, want %s", got.Kind, tt.want.Kind)
			}
			if got.CPUPercent != tt.want.CPUPercent {
				t.Errorf("CPUPercent = %f, want %f", got.CPUPercent, tt.want.CPUPercent)
			}
			if got.CPULimitCores != tt.want.CPULimitCores {
				t.Errorf("CPULimitCores = %d, want %d", got.CPULimitCores, tt.want.CPULimitCores)
			}
			if got.MemoryBytes != tt.want.MemoryBytes {
				t.Errorf("MemoryBytes = %d, want %d", got.MemoryBytes, tt.want.MemoryBytes)
			}
			if got.MemoryLimit != tt.want.MemoryLimit {
				t.Errorf("MemoryLimit = %d, want %d", got.MemoryLimit, tt.want.MemoryLimit)
			}
			if got.NetRxBytes != tt.want.NetRxBytes {
				t.Errorf("NetRxBytes = %d, want %d", got.NetRxBytes, tt.want.NetRxBytes)
			}
			if got.NetTxBytes != tt.want.NetTxBytes {
				t.Errorf("NetTxBytes = %d, want %d", got.NetTxBytes, tt.want.NetTxBytes)
			}
		})
	}
}

func TestHasCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"ls exists", "ls", true},
		{"cat exists", "cat", true},
		{"nonexistent", "this-command-does-not-exist-12345", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("hasCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestCollectProxmoxGuests_NoPvesh(t *testing.T) {
	if hasCommand("pvesh") {
		t.Skip("pvesh is available, cannot test missing command path")
	}

	ctx := context.Background()
	result, err := collectProxmoxGuests(ctx)
	if err != nil {
		t.Errorf("Expected nil error when pvesh missing, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result when pvesh missing, got: %v", result)
	}
}

func TestCollectProxmoxGuests_Integration(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	// Reset cached node for clean test
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()
	result, err := collectProxmoxGuests(ctx)
	if err != nil {
		t.Fatalf("collectProxmoxGuests failed: %v", err)
	}

	t.Logf("Found %d Proxmox guests", len(result))

	for _, c := range result {
		t.Logf("Guest: %s (%s) [%s/%s]", c.Name, c.ID, c.Source, c.Kind)
		t.Logf("  State: %s", c.State)
		t.Logf("  CPU: %.2f%% (%d cores)", c.CPUPercent, c.CPULimitCores)
		t.Logf("  Memory: %d / %d bytes", c.MemoryBytes, c.MemoryLimit)
		t.Logf("  Network: RX=%d TX=%d bytes", c.NetRxBytes, c.NetTxBytes)

		if c.Source != "proxmox" {
			t.Errorf("Source should be 'proxmox', got %s", c.Source)
		}
		if c.Kind != "lxc" && c.Kind != "vm" {
			t.Errorf("Kind should be 'lxc' or 'vm', got %s", c.Kind)
		}
		if c.ID == "" {
			t.Error("ID should not be empty")
		}
		if c.Name == "" {
			t.Error("Name should not be empty")
		}
	}
}

func TestLocalProxmoxNode_Integration(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	// Reset cache
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		t.Fatalf("localProxmoxNode failed: %v", err)
	}

	if node == "" {
		t.Error("Node name should not be empty")
	}

	t.Logf("Resolved Proxmox node: %s", node)

	// Second call should use cache
	node2, err2 := localProxmoxNode(ctx)
	if err2 != nil {
		t.Fatalf("Second localProxmoxNode call failed: %v", err2)
	}
	if node2 != node {
		t.Errorf("Cached node mismatch: %s vs %s", node, node2)
	}
}

func TestProxmoxList_Integration(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	// Reset cache
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}

	t.Run("LXC List", func(t *testing.T) {
		rows, err := proxmoxList(ctx, node, kindLXC)
		if err != nil {
			t.Fatalf("proxmoxList(lxc) failed: %v", err)
		}
		t.Logf("Found %d LXC containers", len(rows))
		for _, r := range rows {
			t.Logf("  VMID=%d Name=%s Status=%s", r.VMID, r.Name, r.Status)
		}
	})

	t.Run("VM List", func(t *testing.T) {
		rows, err := proxmoxList(ctx, node, kindVM)
		if err != nil {
			t.Fatalf("proxmoxList(vm) failed: %v", err)
		}
		t.Logf("Found %d VMs", len(rows))
		for _, r := range rows {
			t.Logf("  VMID=%d Name=%s Status=%s", r.VMID, r.Name, r.Status)
		}
	})

	t.Run("Invalid Kind", func(t *testing.T) {
		_, err := proxmoxList(ctx, node, "invalid")
		if err == nil {
			t.Error("Expected error for invalid kind")
		}
	})
}

func TestProxmoxStatus_Integration(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	// Reset cache
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}

	// Get list of LXCs first
	lxcs, err := proxmoxList(ctx, node, kindLXC)
	if err != nil {
		t.Fatalf("proxmoxList failed: %v", err)
	}

	if len(lxcs) == 0 {
		t.Skip("No LXC containers to test status")
	}

	vmid := strconv.Itoa(lxcs[0].VMID)
	resource, ok := proxmoxStatus(ctx, node, kindLXC, vmid)

	if !ok {
		t.Fatalf("proxmoxStatus failed for VMID %s", vmid)
	}

	t.Logf("Status for VMID %s:", vmid)
	t.Logf("  Name: %s", resource.Name)
	t.Logf("  Status: %s", resource.Status)
	t.Logf("  CPU: %f (%d cores)", resource.CPU, resource.CPUs)
	t.Logf("  Memory: %d / %d", resource.Mem, resource.MaxMem)
}

func TestProxmoxStatus_InvalidVMID(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	// Reset cache
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}

	// Use an unlikely VMID
	_, ok := proxmoxStatus(ctx, node, kindLXC, "999999")
	if ok {
		t.Error("Expected false for non-existent VMID")
	}
}

func TestCollectProxmoxGuests_ContextCancel(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := collectProxmoxGuests(ctx)
	// Should handle cancelled context gracefully
	t.Logf("collectProxmoxGuests with cancelled context: %v", err)
}

func TestCollectProxmoxGuests_Timeout(t *testing.T) {
	if !hasCommand("pvesh") {
		t.Skip("pvesh not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond) // Ensure timeout

	_, err := collectProxmoxGuests(ctx)
	// Should handle timeout gracefully
	t.Logf("collectProxmoxGuests with timeout: %v", err)
}

func TestCollectContainers_Integration(t *testing.T) {
	ctx := context.Background()
	metrics, err := CollectContainers(ctx)
	if err != nil {
		t.Fatalf("CollectContainers failed: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}

	listMetric, ok := metrics[0].(protocol.ContainerListMetric)
	if !ok {
		t.Fatalf("Expected ContainerListMetric, got %T", metrics[0])
	}

	t.Logf("Total containers from all sources: %d", len(listMetric.Containers))

	// Count by source
	sourceCounts := make(map[string]int)
	kindCounts := make(map[string]int)
	for _, c := range listMetric.Containers {
		sourceCounts[c.Source]++
		kindCounts[c.Kind]++
	}

	t.Logf("By source: %v", sourceCounts)
	t.Logf("By kind: %v", kindCounts)
}

// Benchmarks

func BenchmarkMapProxmoxStatus(b *testing.B) {
	resource := proxmoxResource{
		VMID:   100,
		Name:   "test-container",
		Status: "running",
		CPU:    0.25,
		CPUs:   4,
		Mem:    1073741824,
		MaxMem: 2147483648,
		NetIn:  1000000,
		NetOut: 2000000,
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = mapProxmoxStatus(resource, kindLXC)
	}
}

func BenchmarkHasCommand_Exists(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = hasCommand("ls")
	}
}

func BenchmarkHasCommand_NotExists(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = hasCommand("this-does-not-exist-12345")
	}
}

func BenchmarkLocalProxmoxNode_Cached(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	// Prime the cache
	ctx := context.Background()
	_, _ = localProxmoxNode(ctx)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = localProxmoxNode(ctx)
	}
}

func BenchmarkProxmoxList_LXC(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		b.Fatalf("Failed to get node: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = proxmoxList(ctx, node, kindLXC)
	}
}

func BenchmarkProxmoxList_VM(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	ctx := context.Background()
	node, err := localProxmoxNode(ctx)
	if err != nil {
		b.Fatalf("Failed to get node: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = proxmoxList(ctx, node, kindVM)
	}
}

func BenchmarkCollectProxmoxGuests(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = collectProxmoxGuests(ctx)
	}
}

func BenchmarkCollectContainers(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		_, _ = CollectContainers(ctx)
	}
}

// Mock benchmarks for parallel processing simulation

func BenchmarkCollectProxmoxKind_Mock25(b *testing.B) {
	benchmarkCollectProxmoxKindMock(b, 25)
}

func BenchmarkCollectProxmoxKind_Mock100(b *testing.B) {
	benchmarkCollectProxmoxKindMock(b, 100)
}

func benchmarkCollectProxmoxKindMock(b *testing.B, n int) {
	rows := make([]proxmoxListRow, n)
	for i := range rows {
		rows[i] = proxmoxListRow{
			VMID:   100 + i,
			Name:   "test-" + strconv.Itoa(i),
			Status: "running",
		}
	}

	// This only benchmarks the parallel collection logic, not actual pvesh calls
	b.ReportAllocs()
	for b.Loop() {
		type result struct {
			m  protocol.ContainerMetric
			ok bool
		}

		results := make(chan result, len(rows))
		sem := make(chan struct{}, proxmoxConcurrency)

		for _, row := range rows {
			go func(row proxmoxListRow) {
				sem <- struct{}{}
				defer func() { <-sem }()

				// Simulate successful status fetch
				r := proxmoxResource{
					VMID:   row.VMID,
					Name:   row.Name,
					Status: row.Status,
					CPU:    0.25,
					CPUs:   4,
					Mem:    1073741824,
					MaxMem: 2147483648,
				}
				results <- result{
					m:  mapProxmoxStatus(r, kindLXC),
					ok: true,
				}
			}(row)
		}

		var out []protocol.ContainerMetric
		for range rows {
			r := <-results
			if r.ok {
				out = append(out, r.m)
			}
		}
		_ = out
	}
}

func BenchmarkCollectProxmoxGuests_Real(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	// Reset cache
	cachedNodeOnce = sync.Once{}
	cachedNode = ""
	cachedNodeErr = nil

	ctx := context.Background()

	// Report guest count
	result, _ := collectProxmoxGuests(ctx)
	b.Logf("Benchmarking with %d guests", len(result))

	b.ResetTimer()
	for b.Loop() {
		// Reset cache each iteration to measure full cost
		cachedNodeOnce = sync.Once{}
		cachedNode = ""
		cachedNodeErr = nil

		_, _ = collectProxmoxGuests(ctx)
	}
}

func BenchmarkCollectProxmoxGuests_Real_CachedNode(b *testing.B) {
	if !hasCommand("pvesh") {
		b.Skip("pvesh not available")
	}

	ctx := context.Background()

	// Prime cache
	_, _ = localProxmoxNode(ctx)

	// Report guest count
	result, _ := collectProxmoxGuests(ctx)
	b.Logf("Benchmarking with %d guests (node cached)", len(result))

	b.ResetTimer()
	for b.Loop() {
		_, _ = collectProxmoxGuests(ctx)
	}
}
