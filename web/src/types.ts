// Auth

export interface User {
    username: string;
    role: string;
}

// Agents

export interface Agent {
    id: string;
    hostname: string;
    os: string;
    platform: string;
    arch: string;
    cpu_model: string;
    cpu_cores: number;
    ram_total: number;
    registered_at: string;
    last_seen: string | null;
}

// Overview (agents LEFT JOIN current_metrics)

export interface OverviewAgent {
    id: string;
    hostname: string;
    os: string;
    platform: string;
    arch: string;
    cpu_cores: number;
    last_seen: string | null;
    cpu_usage: number | null;
    load_normalized: number | null;
    ram_percent: number | null;
    swap_percent: number | null;
    disk_max_percent: number | null;
    net_rx_bytes: number | null;
    net_tx_bytes: number | null;
    max_temp: number | null;
    uptime: number | null;
    process_count: number | null;
    reboot_required: boolean | null;
    updated_at: string | null;
}

// Time-series metrics

export interface CPUMetric {
    time: string;
    agent_id: string;
    usage: number;
    core_usages: number[] | null;
    load_1m: number;
    load_5m: number;
    load_15m: number;
    iowait: number;
}

export interface MemoryMetric {
    time: string;
    agent_id: string;
    ram_total: number;
    ram_used: number;
    ram_available: number;
    ram_percent: number;
    swap_total: number;
    swap_used: number;
    swap_percent: number;
}

export interface DiskMetric {
    time: string;
    agent_id: string;
    device: string;
    mountpoint: string;
    filesystem: string;
    disk_type: string;
    total_bytes: number;
    used_bytes: number;
    free_bytes: number;
    used_percent: number;
    inodes_total: number;
    inodes_used: number;
    inodes_percent: number;
}

export interface DiskIOMetric {
    time: string;
    agent_id: string;
    device: string;
    read_bytes: number;
    write_bytes: number;
    read_ops: number;
    write_ops: number;
    read_latency: number;
    write_latency: number;
    io_in_progress: number;
}

export interface NetworkMetric {
    time: string;
    agent_id: string;
    interface: string;
    mac: string;
    mtu: number;
    speed: number;
    rx_bytes: number;
    rx_packets: number;
    rx_errors: number;
    rx_drops: number;
    tx_bytes: number;
    tx_packets: number;
    tx_errors: number;
    tx_drops: number;
}

export interface TemperatureMetric {
    time: string;
    agent_id: string;
    sensor: string;
    temperature: number;
    max_temp: number;
}

export interface SystemMetric {
    time: string;
    agent_id: string;
    uptime: number;
    process_count: number;
    user_count: number;
    boot_time: string;
}

export interface ContainerMetric {
    time: string;
    agent_id: string;
    container_id: string;
    name: string;
    image: string;
    state: string;
    source: string;
    kind: string;
    cpu_percent: number;
    cpu_cores: number;
    memory_bytes: number;
    memory_limit: number;
    net_rx_bytes: number;
    net_tx_bytes: number;
}

export interface WifiMetric {
    time: string;
    agent_id: string;
    interface: string;
    ssid: string;
    bssid: string;
    frequency_mhz: number;
    signal_dbm: number;
    noise_dbm: number;
    bitrate_mbps: number;
    link_quality: number;
}

export interface PiMetric {
    time: string;
    agent_id: string;
    metric_type: string;
    arm_freq_hz: number;
    core_freq_hz: number;
    gpu_freq_hz: number;
    core_volts: number;
    sdram_c_volts: number;
    sdram_i_volts: number;
    sdram_p_volts: number;
    soft_temp_limit: number;
    throttled: boolean;
    under_voltage: boolean;
    freq_capped: boolean;
    undervoltage_occurred: boolean;
    freq_cap_occurred: boolean;
    throttled_occurred: boolean;
    soft_temp_limit_occurred: boolean;
    gpu_mem_total: number;
    gpu_mem_used: number;
    gpu_temp: number;
}

// Current State

export interface Process {
    agent_id: string;
    pid: number;
    name: string;
    cpu_percent: number;
    mem_percent: number;
    mem_rss: number;
    status: string;
    threads: number;
    updated_at: string;
}

export interface Service {
    agent_id: string;
    name: string;
    status: string;
    sub_status: string;
    updated_at: string
}

export interface Application {
    agent_id: string;
    name: string;
    version: string;
    updated_at: string;
}

export interface Updates {
    agent_id: string;
    pending_count: number;
    security_count: number;
    reboot_required: boolean;
    package_manager: string;
    updated_at: string;
}

// UI Types

export type TimeRange = "5m" | "15m" | "1h" | "6h" | "24h" | "7d" | "30d"
export type ProcessSort = "cpu" | "memory";
export type Page = "overview" | "agents" | "admin";

/* Unified time selection - either a quick preset or a custom start/end. */
export type RangeSelection =
    | { type: "quick"; range: TimeRange }
    | { type: "custom"; start: string; end: string }; // ISO 8601 strings

// Provisioning

export interface PlatformInfo {
    os: string;
    arch: string;
    variant?: string;
    label: string;
    filename: string;
}

export interface AgentConfig {
    server: string;
    token: string;
}

export interface InstallInstructions {
    type: string;
    content: string;
    steps: string;
}

export interface ProvisionResponse {
    token: string;
    expires_at: string;
    platform: string;
    download_url: string;
    config: AgentConfig;
    install: InstallInstructions;
}