package metrics

import (
	"encoding/json"
	"time"
)

// Metric is implemented by all metric types
type Metric interface {
	MetricType() string
}

// ProcessListMetric holds all proccesses from a single collection
type ProcessListMetric struct {
	Processes []ProcessMetric `json:"processes"`
}

// Envelope wraps any metric with metadata for transmission
type Envelope struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Hostname  string    `json:"hostname"`
	Data      Metric    `json:"data"`
}

// MarshalJSON ensured proper serialization with the concrete type
func (e Envelope) MarshalJSON() ([]byte, error) {
	type Alias Envelope
	return json.Marshal(&struct {
		Alias
		Data any `json:"data"`
	}{
		Alias: Alias(e),
		Data:  e.Data,
	})
}

// Impelement the interface on each metric type
func (CPUMetric) MetricType() string         { return "cpu" }
func (MemoryMetric) MetricType() string      { return "memory" }
func (DiskMetric) MetricType() string        { return "disk" }
func (NetworkMetric) MetricType() string     { return "network" }
func (TemperatureMetric) MetricType() string { return "temperature" }
func (SystemMetric) MetricType() string      { return "system" }
func (DiskIOMetric) MetricType() string      { return "disk_io" }
func (ProcessMetric) MetricType() string     { return "process" }
func (ProcessListMetric) MetricType() string { return "process_list" }
func (ThrottleMetric) MetricType() string    { return "throttle" }
func (ClockMetric) MetricType() string       { return "clock" }
func (VoltageMetric) MetricType() string     { return "voltage" }
func (WiFiMetric) MetricType() string        { return "wifi" }
func (GPUMetric) MetricType() string         { return "gpu" }
