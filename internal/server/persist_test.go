// persist_test.go
package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

func TestPersistMetric(t *testing.T) {
	agentID := "00000000-0000-0000-0000-000000000001"
	ts := time.Now()
	ctx := context.Background()

	tests := []struct {
		name      string
		metric    protocol.Metric
		checkMock func(t *testing.T, m *MockDB)
	}{
		{
			name:   "CPU",
			metric: &protocol.CPUMetric{Usage: 55.5, CoreUsage: []float64{50, 60}, LoadAvg1: 1.0, LoadAvg5: 0.8, LoadAvg15: 0.5, IOWait: 2.1},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertCPUCount != 1 {
					t.Errorf("InsertCPU called %d times, want 1", m.InsertCPUCount)
				}
			},
		},
		{
			name:   "CPU with nil CoreUsage",
			metric: &protocol.CPUMetric{Usage: 10.0, CoreUsage: nil},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertCPUCount != 1 {
					t.Errorf("InsertCPU called %d times, want 1", m.InsertCPUCount)
				}
			},
		},
		{
			name:   "Memory",
			metric: &protocol.MemoryMetric{Total: 16000000000, Used: 8000000000, Available: 8000000000, UsedPct: 50.0, SwapTotal: 4000000000, SwapUsed: 1000000000, SwapPct: 25.0},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertMemoryCount != 1 {
					t.Errorf("InsertMemory called %d times, want 1", m.InsertMemoryCount)
				}
			},
		},
		{
			name:   "Disk",
			metric: &protocol.DiskMetric{Device: "/dev/sda1", Mountpoint: "/", Filesystem: "ext4", Type: "SSD", Total: 500000000000, Used: 250000000000, Available: 250000000000, UsedPct: 50.0, InodesTotal: 1000000, InodesUsed: 500000, InodesPct: 50.0},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertDiskCount != 1 {
					t.Errorf("InsertDisk called %d times, want 1", m.InsertDiskCount)
				}
			},
		},
		{
			name:   "DiskIO",
			metric: &protocol.DiskIOMetric{Device: "sda", ReadBytes: 1024, WriteBytes: 2048, ReadOps: 10, WriteOps: 20, ReadTime: 5, WriteTime: 10, InProgress: 2},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertDiskIOCount != 1 {
					t.Errorf("InsertDiskIO called %d times, want 1", m.InsertDiskIOCount)
				}
			},
		},
		{
			name:   "Network",
			metric: &protocol.NetworkMetric{Interface: "eth0", TxBytes: 1000, RxBytes: 2000},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertNetworkCount != 1 {
					t.Errorf("InsertNetwork called %d times, want 1", m.InsertNetworkCount)
				}
			},
		},
		{
			name:   "Temperature",
			metric: &protocol.TemperatureMetric{Sensor: "coretemp", Temp: 65.5, Max: &[]float64{100.0}[0]},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertTemperatureCount != 1 {
					t.Errorf("InsertTemperature called %d times, want 1", m.InsertTemperatureCount)
				}
			},
		},
		{
			name:   "WiFi",
			metric: &protocol.WiFiMetric{},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertWifiCount != 1 {
					t.Errorf("InsertWifi called %d times, want 1", m.InsertWifiCount)
				}
			},
		},
		{
			name:   "System",
			metric: &protocol.SystemMetric{Uptime: 86400, Processes: 250, Users: 3, BootTime: 1700000000},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertSystemCount != 1 {
					t.Errorf("InsertSystem called %d times, want 1", m.InsertSystemCount)
				}
			},
		},
		{
			name:   "Container",
			metric: &protocol.ContainerMetric{Name: "nginx", Image: "nginx:latest", State: "running"},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertContainerCount != 1 {
					t.Errorf("InsertContainer called %d times, want 1", m.InsertContainerCount)
				}
			},
		},
		{
			name: "ContainerList",
			metric: &protocol.ContainerListMetric{
				Containers: []protocol.ContainerMetric{
					{Name: "nginx", State: "running"},
					{Name: "redis", State: "running"},
					{Name: "postgres", State: "running"},
				},
			},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertContainerCount != 3 {
					t.Errorf("InsertContainer called %d times, want 3", m.InsertContainerCount)
				}
			},
		},
		{
			name: "ProcessList",
			metric: &protocol.ProcessListMetric{
				Processes: []protocol.ProcessMetric{
					{Pid: 1, Name: "init", CPUPercent: 0.1, MemPercent: 0.5},
					{Pid: 100, Name: "nginx", CPUPercent: 5.0, MemPercent: 2.0},
				},
			},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.UpsertProcessCount != 2 {
					t.Errorf("UpsertProcess called %d times, want 2", m.UpsertProcessCount)
				}
			},
		},
		{
			name: "ServiceList",
			metric: &protocol.ServiceListMetric{
				Services: []protocol.ServiceMetric{
					{Name: "sshd", Status: "running", SubStatus: "active"},
					{Name: "nginx", Status: "running", SubStatus: "active"},
				},
			},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.UpsertServiceCount != 2 {
					t.Errorf("UpsertService called %d times, want 2", m.UpsertServiceCount)
				}
			},
		},
		{
			name: "ApplicationList",
			metric: &protocol.ApplicationListMetric{
				Applications: []protocol.Application{
					{Name: "vim", Version: "9.0"},
					{Name: "git", Version: "2.40"},
				},
			},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.UpsertApplicationCount != 2 {
					t.Errorf("UpsertApplication called %d times, want 2", m.UpsertApplicationCount)
				}
			},
		},
		{
			name:   "Clock (Pi)",
			metric: &protocol.ClockMetric{ArmFreq: 1500000000, CoreFreq: 500000000, GPUFreq: 400000000},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertPiCount != 1 {
					t.Errorf("InsertPi called %d times, want 1", m.InsertPiCount)
				}
			},
		},
		{
			name:   "Voltage (Pi)",
			metric: &protocol.VoltageMetric{Core: 1.2, SDRamC: 1.2, SDRamI: 1.2, SDRamP: 1.2},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertPiCount != 1 {
					t.Errorf("InsertPi called %d times, want 1", m.InsertPiCount)
				}
			},
		},
		{
			name:   "Throttle (Pi)",
			metric: &protocol.ThrottleMetric{Throttled: true, Undervoltage: false, ArmFreqCapped: true},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertPiCount != 1 {
					t.Errorf("InsertPi called %d times, want 1", m.InsertPiCount)
				}
			},
		},
		{
			name:   "GPU (Pi)",
			metric: &protocol.GPUMetric{MemoryTotal: 128000000, MemoryUsed: 64000000},
			checkMock: func(t *testing.T, m *MockDB) {
				if m.InsertPiCount != 1 {
					t.Errorf("InsertPi called %d times, want 1", m.InsertPiCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDB()
			s := &Server{DB: mock}

			s.persistMetric(ctx, agentID, ts, tt.metric)

			tt.checkMock(t, mock)
		})
	}
}

func TestPersistMetric_NilDB(t *testing.T) {
	s, _, _, mock := newTestServer()
	s.DB = nil
	_ = mock

	// Should not panic
	s.persistMetric(context.Background(), "00000000-0000-0000-0000-000000000001", time.Now(), &protocol.CPUMetric{})
}

func TestPersistMetric_UnknownMetricType(t *testing.T) {
	s, _, _, mock := newTestServer()

	s.persistMetric(context.Background(), "00000000-0000-0000-0000-000000000001", time.Now(), unknownMetric{})

	// Verify no DB methods were called
	if mock.InsertCPUCount+mock.InsertMemoryCount+mock.InsertDiskCount+mock.InsertDiskIOCount+
		mock.InsertNetworkCount+mock.InsertTemperatureCount+mock.InsertWifiCount+
		mock.InsertSystemCount+mock.InsertContainerCount+mock.InsertPiCount+
		mock.UpsertProcessCount+mock.UpsertServiceCount+mock.UpsertApplicationCount != 0 {
		t.Error("unexpected DB call for unknown metric type")
	}
}

// unknownMetric is a test-only type to exercise the default branch.
type unknownMetric struct{}

func (unknownMetric) MetricType() string { return "unknown_test" }

func TestPersistMetric_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.Err = errors.New("db connection lost")

	// Should not panic — just logs the error
	s.persistMetric(context.Background(), "00000000-0000-0000-0000-000000000001", time.Now(), &protocol.CPUMetric{Usage: 50.0})

	if mock.InsertCPUCount != 1 {
		t.Errorf("InsertCPU should still be called, got %d", mock.InsertCPUCount)
	}
}

func TestPersistMetric_ProcessListDBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.Err = errors.New("upsert failed")

	metric := &protocol.ProcessListMetric{
		Processes: []protocol.ProcessMetric{
			{Pid: 1, Name: "init"},
			{Pid: 2, Name: "kthreadd"},
		},
	}

	// Should not panic — logs per-process errors and continues
	s.persistMetric(context.Background(), "00000000-0000-0000-0000-000000000001", time.Now(), metric)

	// Both upserts should still be attempted
	if mock.UpsertProcessCount != 2 {
		t.Errorf("UpsertProcess called %d times, want 2", mock.UpsertProcessCount)
	}
}

func TestPersistMetric_ContainerListEmpty(t *testing.T) {
	s, _, _, mock := newTestServer()

	s.persistMetric(context.Background(), "00000000-0000-0000-0000-000000000001", time.Now(), &protocol.ContainerListMetric{
		Containers: []protocol.ContainerMetric{},
	})

	if mock.InsertContainerCount != 0 {
		t.Errorf("InsertContainer called %d times for empty list, want 0", mock.InsertContainerCount)
	}
}
