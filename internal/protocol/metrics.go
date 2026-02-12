package protocol

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
func (CPUMetric) MetricType() string             { return "cpu" }
func (MemoryMetric) MetricType() string          { return "memory" }
func (DiskMetric) MetricType() string            { return "disk" }
func (NetworkMetric) MetricType() string         { return "network" }
func (TemperatureMetric) MetricType() string     { return "temperature" }
func (SystemMetric) MetricType() string          { return "system" }
func (DiskIOMetric) MetricType() string          { return "disk_io" }
func (ProcessMetric) MetricType() string         { return "process" }
func (ProcessListMetric) MetricType() string     { return "process_list" }
func (ThrottleMetric) MetricType() string        { return "throttle" }
func (ClockMetric) MetricType() string           { return "clock" }
func (VoltageMetric) MetricType() string         { return "voltage" }
func (WiFiMetric) MetricType() string            { return "wifi" }
func (GPUMetric) MetricType() string             { return "gpu" }
func (ApplicationListMetric) MetricType() string { return "application_list" }
func (ContainerMetric) MetricType() string       { return "container" }
func (ContainerListMetric) MetricType() string   { return "container_list" }

type CPUMetric struct {
	Usage     float64   `json:"usage"`
	CoreUsage []float64 `json:"cores"`
	IOWait    float64   `json:"iowait"`
	LoadAvg1  float64   `json:"load_1m"`
	LoadAvg5  float64   `json:"load_5m,omitempty"`
	LoadAvg15 float64   `json:"load_15m,omitempty"`
}

type MemoryMetric struct {
	Total     uint64  `json:"ram_total"`
	Used      uint64  `json:"ram_used"`
	Available uint64  `json:"ram_available"`
	UsedPct   float64 `json:"ram_used_pct"`
	SwapTotal uint64  `json:"swap_total"`
	SwapUsed  uint64  `json:"swap_used"`
	SwapPct   float64 `json:"swap_pct"`
}

type DiskMetric struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Filesystem  string  `json:"filesystem"`
	Type        string  `json:"disk_type"`
	Total       uint64  `json:"disk_total"`
	Used        uint64  `json:"disk_used"`
	Available   uint64  `json:"disk_available"`
	UsedPct     float64 `json:"disk_used_pct"`
	InodesTotal uint64  `json:"inodes_total,omitempty"`
	InodesUsed  uint64  `json:"inodes_used,omitempty"`
	InodesPct   float64 `json:"inodes_pct,omitempty"`
}

type NetworkMetric struct {
	Interface string `json:"interface"`
	MAC       string `json:"mac_address"`
	MTU       uint32 `json:"mtu"`
	Speed     uint64 `json:"speed"`
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDrops   uint64 `json:"rx_drops"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDrops   uint64 `json:"tx_drops"`
}

type TemperatureMetric struct {
	Sensor string  `json:"sensor"`
	Temp   float64 `json:"temperature"`
	Max    float64 `json:"max_temp"`
}

type SystemMetric struct {
	Uptime    uint64 `json:"uptime"`
	Processes int    `json:"processes"`
	Users     int    `json:"users"`
	BootTime  uint64 `json:"boot_time"`
}

type DiskIOMetric struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadOps    uint64 `json:"read_ops"`
	WriteOps   uint64 `json:"write_ops"`
	ReadTime   uint64 `json:"read_time_ms"`
	WriteTime  uint64 `json:"write_time_ms"`
	InProgress uint64 `json:"io_in_progress"`
}

type ProcessMetric struct {
	Pid             int        `json:"pid"`
	Name            string     `json:"name"`
	CPUPercent      float64    `json:"cpu_percent"`
	MemPercent      float64    `json:"mem_percent"`
	MemRSS          uint64     `json:"mem_rss"`
	Status          ProcStatus `json:"status"`
	ThreadsTotal    uint32     `json:"threads_total"`
	ThreadsRunning  *uint32    `json:"threads_running,omitempty"`
	ThreadsRunnable *uint32    `json:"threads_runnable,omitempty"`
	ThreadsWaiting  *uint32    `json:"threads_waiting,omitempty"`
}

type ThrottleMetric struct {
	Undervoltage          bool `json:"undervoltage,omitempty"`
	ArmFreqCapped         bool `json:"arm_freq_capped,omitempty"`
	Throttled             bool `json:"throttled,omitempty"`
	SoftTempLimit         bool `json:"soft_temp_limit,omitempty"`
	UndervoltageOccurred  bool `json:"undervoltage_occurred,omitempty"`
	FreqCapOccurred       bool `json:"freq_cap_occurred,omitempty"`
	ThrottledOccurred     bool `json:"throttled_occurred,omitempty"`
	SoftTempLimitOccurred bool `json:"soft_temp_occurred,omitempty"`
}

type ClockMetric struct {
	ArmFreq  uint64 `json:"arm_freq_hz,omitempty"`
	CoreFreq uint64 `json:"core_freq_hz,omitempty"`
	GPUFreq  uint64 `json:"gpu_freq_hz,omitempty"`
}

type VoltageMetric struct {
	Core   float64 `json:"core_volts,omitempty"`
	SDRamC float64 `json:"sdram_c_volts,omitempty"`
	SDRamI float64 `json:"sdram_i_volts,omitempty"`
	SDRamP float64 `json:"sdram_p_volts,omitempty"`
}

type WiFiMetric struct {
	Interface   string  `json:"interface"`
	SSID        string  `json:"ssid"`
	SignalLevel int     `json:"signal_dbm"`
	LinkQuality int     `json:"link_quality"`
	Frequency   float64 `json:"frequency_ghz"`
	BitRate     float64 `json:"bitrate_mbps"`
}

type GPUMetric struct {
	MemoryTotal uint64 `json:"gpu_mem_total,omitempty"`
	MemoryUsed  uint64 `json:"gpu_mem_used,omitempty"`
}

type Application struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Vendor  string `json:"vendor,omitempty"`
}

type ApplicationListMetric struct {
	Applications []Application `json:"applications"`
}

type ContainerMetric struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Image         string  `json:"image,omitempty"`
	State         string  `json:"state"`
	Source        string  `json:"source"` // "docker", "containerd", "proxmox"
	Kind          string  `json:"kind"`   // "container", "lxc", "vm"
	CPUPercent    float64 `json:"cpu_percent"`
	CPULimitCores uint32  `json:"cpu_limit_cores,omitempty"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	MemoryLimit   uint64  `json:"memory_limit,omitempty"`
	NetRxBytes    uint64  `json:"net_rx_bytes,omitempty"`
	NetTxBytes    uint64  `json:"net_tx_bytes,omitempty"`
}

type ContainerListMetric struct {
	Containers []ContainerMetric `json:"containers"`
}
