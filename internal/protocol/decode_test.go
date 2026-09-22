package protocol

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalMetric_AllTypes(t *testing.T) {
	tests := []struct {
		typ      string
		data     string
		wantType string
	}{
		{"cpu", `{"usage": 50.5, "cores": [40, 60]}`, "cpu"},
		{"memory", `{"ram_total": 16000000000, "ram_used": 8000000000}`, "memory"},
		{"disk", `{"device": "/dev/sda1", "mountpoint": "/"}`, "disk"},
		{"disk_io", `{"device": "sda", "read_bytes": 1000}`, "disk_io"},
		{"network", `{"interface": "eth0", "rx_bytes": 1000}`, "network"},
		{"wifi", `{"interface": "wlan0", "ssid": "Test"}`, "wifi"},
		{"clock", `{"arm_freq_hz": 1500000000}`, "clock"},
		{"voltage", `{"core_volts": 1.2}`, "voltage"},
		{"throttle", `{"throttled": true}`, "throttle"},
		{"gpu", `{"gpu_mem_total": 8000000000}`, "gpu"},
		{"system", `{"uptime": 3600, "processes": 100}`, "system"},
		{"process", `{"pid": 1, "name": "init"}`, "process"},
		{"process_list", `{"processes": [{"pid": 1, "name": "init"}]}`, "process_list"},
		{"temperature", `{"sensor": "coretemp", "temperature": 45.5}`, "temperature"},
		{"service", `{"name": "nginx", "status": "active"}`, "service"},
		{"service_list", `{"services": [{"name": "nginx", "status": "active"}]}`, "service_list"},
		{"application_list", `{"applications": [{"name": "vim", "version": "8.0"}]}`, "application_list"},
		{"container", `{"id": "abc123", "name": "nginx", "state": "running"}`, "container"},
		{"container_list", `{"containers": [{"id": "abc123", "name": "nginx"}]}`, "container_list"},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			metric, err := UnmarshalMetric(tt.typ, []byte(tt.data))
			if err != nil {
				t.Fatalf("UnmarshalMetric(%s) error: %v", tt.typ, err)
			}
			if metric.MetricType() != tt.wantType {
				t.Errorf("MetricType() = %s, want %s", metric.MetricType(), tt.wantType)
			}
		})
	}
}

func TestUnmarshalMetric_UnknownType(t *testing.T) {
	_, err := UnmarshalMetric("unknown_type", []byte(`{}`))
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestUnmarshalMetric_InvalidJSON(t *testing.T) {
	_, err := UnmarshalMetric("cpu", []byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalMetric_CPUMetric_Values(t *testing.T) {
	data := `{"usage": 75.5, "cores": [80, 70, 85, 65], "load_1m": 2.5}`
	metric, err := UnmarshalMetric("cpu", []byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cpu, ok := metric.(*CPUMetric)
	if !ok {
		t.Fatal("expected *CPUMetric")
	}

	if cpu.Usage != 75.5 {
		t.Errorf("Usage = %f, want 75.5", cpu.Usage)
	}
	if len(cpu.CoreUsage) != 4 {
		t.Errorf("CoreUsage length = %d, want 4", len(cpu.CoreUsage))
	}
	if cpu.LoadAvg1 != 2.5 {
		t.Errorf("LoadAvg1 = %f, want 2.5", cpu.LoadAvg1)
	}
}

func TestUnmarshalMetric_ProcessListMetric_Values(t *testing.T) {
	data := `{"processes": [
		{"pid": 1, "name": "init", "cpu_percent": 0.1, "mem_percent": 0.5},
		{"pid": 100, "name": "nginx", "cpu_percent": 5.0, "mem_percent": 2.0}
	]}`

	metric, err := UnmarshalMetric("process_list", []byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl, ok := metric.(*ProcessListMetric)
	if !ok {
		t.Fatal("expected *ProcessListMetric")
	}

	if len(pl.Processes) != 2 {
		t.Errorf("Processes length = %d, want 2", len(pl.Processes))
	}
	if pl.Processes[0].Pid != 1 {
		t.Errorf("Processes[0].Pid = %d, want 1", pl.Processes[0].Pid)
	}
	if pl.Processes[1].Name != "nginx" {
		t.Errorf("Processes[1].Name = %s, want nginx", pl.Processes[1].Name)
	}
}

func TestUnmarshalMetric_ContainerListMetric_Values(t *testing.T) {
	data := `{"containers": [
		{"id": "abc123", "name": "nginx", "state": "running", "source": "docker", "kind": "container", "cpu_percent": 15.5},
		{"id": "def456", "name": "redis", "state": "running", "source": "docker", "kind": "container", "memory_bytes": 100000000}
	]}`

	metric, err := UnmarshalMetric("container_list", []byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cl, ok := metric.(*ContainerListMetric)
	if !ok {
		t.Fatal("expected *ContainerListMetric")
	}

	if len(cl.Containers) != 2 {
		t.Errorf("Containers length = %d, want 2", len(cl.Containers))
	}
	if cl.Containers[0].CPUPercent != 15.5 {
		t.Errorf("Containers[0].CPUPercent = %f, want 15.5", cl.Containers[0].CPUPercent)
	}
	if cl.Containers[1].MemoryBytes != 100000000 {
		t.Errorf("Containers[1].MemoryBytes = %d, want 100000000", cl.Containers[1].MemoryBytes)
	}
}

func BenchmarkUnmarshalMetric_CPU(b *testing.B) {
	data := []byte(`{"usage": 75.5, "cores": [80, 70, 85, 65, 90, 60, 75, 80], "load_1m": 2.5, "load_5m": 2.0, "load_15m": 1.5}`)

	b.ReportAllocs()
	for b.Loop() {
		_, _ = UnmarshalMetric("cpu", data)
	}
}

func BenchmarkUnmarshalMetric_ProcessList_Small(b *testing.B) {
	procs := make([]ProcessMetric, 10)
	for i := range procs {
		procs[i] = ProcessMetric{Pid: i, Name: "process", CPUPercent: 1.0}
	}
	data, _ := json.Marshal(ProcessListMetric{Processes: procs})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = UnmarshalMetric("process_list", data)
	}
}

func BenchmarkUnmarshalMetric_ProcessList_Large(b *testing.B) {
	procs := make([]ProcessMetric, 200)
	for i := range procs {
		procs[i] = ProcessMetric{Pid: i, Name: "process", CPUPercent: 1.0, MemPercent: 0.5, MemRSS: 1000000}
	}
	data, _ := json.Marshal(ProcessListMetric{Processes: procs})

	b.Logf("Payload size: %d bytes", len(data))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = UnmarshalMetric("process_list", data)
	}
}

func BenchmarkUnmarshalMetric_ContainerList(b *testing.B) {
	containers := make([]ContainerMetric, 20)
	for i := range containers {
		containers[i] = ContainerMetric{
			ID:          "abc123",
			Name:        "container",
			State:       "running",
			Source:      "docker",
			Kind:        "container",
			CPUPercent:  10.0,
			MemoryBytes: 100000000,
		}
	}
	data, _ := json.Marshal(ContainerListMetric{Containers: containers})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = UnmarshalMetric("container_list", data)
	}
}
