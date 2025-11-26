package metrics

type CPUMetric struct {
	Usage     float64   `json:"usage"`
	CoreUsage []float64 `json:"cores"`
	LoadAvg1  float64   `json:"load_1m"`
	LoadAvg5  float64   `json:"load_5m"`
	LoadAvg15 float64   `json:"load_15m"`
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
	InodesTotal uint64  `json:"inodes_total"`
	InodesUsed  uint64  `json:"inodes_used"`
	InodesPct   float64 `json:"inodes_pct"`
}

type NetworkMetric struct {
	Interface   string `json:"interface"`
	BytesRcvd   uint64 `json:"bytes_rcvd"`
	BytesSent   uint64 `json:"bytes_sent"`
	PacketsRcvd uint64 `json:"packets_rcvd"`
	PacketsSent uint64 `json:"packets_sent"`
	ErrorsRcvd  uint64 `json:"errors_rcvd"`
	ErrorsSent  uint64 `json:"errors_sent"`
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
	Pid        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemRSS     uint64  `json:"mem_rss"`
	Status     string  `json:"status"`
}

type ThrottleMetric struct {
	Undervoltage          bool `json:"undervoltage"`
	ArmFreqCapped         bool `json:"arm_freq_capped"`
	Throttled             bool `json:"throttled"`
	SoftTempLimit         bool `json:"soft_temp_limit"`
	UndervoltageOccurred  bool `json:"undervoltage_occurred"`
	FreqCapOccurred       bool `json:"freq_cap_occurred"`
	ThrottledOccurred     bool `json:"throttled_occurred"`
	SoftTempLimitOccurred bool `json:"soft_temp_occurred"`
}

type ClockMetric struct {
	ArmFreq  uint64 `json:"arm_freq_hz"`
	CoreFreq uint64 `json:"core_freq_hz"`
	GPUFreq  uint64 `json:"gpu_freq_hz"`
}

type VoltageMetric struct {
	Core   float64 `json:"core_volts"`
	SDRamC float64 `json:"sdram_c_volts"`
	SDRamI float64 `json:"sdram_i_volts"`
	SDRamP float64 `json:"sdram_p_volts"`
}

type WiFiMetric struct {
	SSID        string  `json:"ssid"`
	SignalLevel int     `json:"signal_dbm"`
	LinkQuality int     `json:"link_quality"`
	Frequency   float64 `json:"frequency_ghz"`
	BitRate     float64 `json:"bitrate_mbps"`
}

type GPUMetric struct {
	MemoryTotal uint64 `json:"gpu_mem_total"`
	MemoryUsed  uint64 `json:"gpu_mem_used"`
}
