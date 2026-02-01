package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelope_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		env   Envelope
		check func(t *testing.T, data map[string]any)
	}{
		{
			name: "CPU metric",
			env: Envelope{
				Type:      "cpu",
				Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Hostname:  "test-host",
				Data:      CPUMetric{Usage: 75.5, CoreUsage: []float64{80, 70}},
			},
			check: func(t *testing.T, data map[string]any) {
				if data["type"] != "cpu" {
					t.Errorf("type: got %v, want cpu", data["type"])
				}
				if data["hostname"] != "test-host" {
					t.Errorf("hostname: got %v, want test-host", data["hostname"])
				}
				d, ok := data["data"].(map[string]any)
				if !ok {
					t.Fatal("data field not a map")
				}
				if d["usage"] != 75.5 {
					t.Errorf("usage: got %v, want 75.5", d["usage"])
				}
			},
		},
		{
			name: "Memory metric",
			env: Envelope{
				Type:      "memory",
				Timestamp: time.Now(),
				Hostname:  "test-host",
				Data:      MemoryMetric{Total: 16000000000, Used: 8000000000, UsedPct: 50.0},
			},
			check: func(t *testing.T, data map[string]any) {
				d := data["data"].(map[string]any)
				if d["ram_total"] != float64(16000000000) {
					t.Errorf("ram_total: got %v, want 16000000000", d["ram_total"])
				}
			},
		},
		{
			name: "ProcessList metric",
			env: Envelope{
				Type:      "process_list",
				Timestamp: time.Now(),
				Hostname:  "test-host",
				Data: ProcessListMetric{
					Processes: []ProcessMetric{
						{Pid: 1, Name: "init", CPUPercent: 0.1},
						{Pid: 2, Name: "kthreadd", CPUPercent: 0.0},
					},
				},
			},
			check: func(t *testing.T, data map[string]any) {
				d := data["data"].(map[string]any)
				procs := d["processes"].([]any)
				if len(procs) != 2 {
					t.Errorf("expected 2 processes, got %d", len(procs))
				}
			},
		},
		{
			name: "Nil data",
			env: Envelope{
				Type:      "unknown",
				Timestamp: time.Now(),
				Hostname:  "test-host",
				Data:      nil,
			},
			check: func(t *testing.T, data map[string]any) {
				if data["data"] != nil {
					t.Errorf("data should be nil, got %v", data["data"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.env)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var data map[string]any
			if err := json.Unmarshal(b, &data); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			tt.check(t, data)
		})
	}
}

func TestMetricType(t *testing.T) {
	tests := []struct {
		metric   Metric
		expected string
	}{
		{CPUMetric{}, "cpu"},
		{MemoryMetric{}, "memory"},
		{DiskMetric{}, "disk"},
		{NetworkMetric{}, "network"},
		{TemperatureMetric{}, "temperature"},
		{SystemMetric{}, "system"},
		{DiskIOMetric{}, "disk_io"},
		{ProcessMetric{}, "process"},
		{ProcessListMetric{}, "process_list"},
		{ThrottleMetric{}, "throttle"},
		{ClockMetric{}, "clock"},
		{VoltageMetric{}, "voltage"},
		{WiFiMetric{}, "wifi"},
		{GPUMetric{}, "gpu"},
		{ApplicationListMetric{}, "application_list"},
		{ContainerMetric{}, "container"},
		{ContainerListMetric{}, "container_list"},
		{ServiceMetric{}, "service"},
		{ServiceListMetric{}, "service_list"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.metric.MetricType(); got != tt.expected {
				t.Errorf("MetricType() = %s, want %s", got, tt.expected)
			}
		})
	}
}
