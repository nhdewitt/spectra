-- name: GetCPURange :many
SELECT time, agent_id, usage, core_usages, load_1m, load_5m, load_15m, iowait
FROM metrics_cpu
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetMemoryRange :many
SELECT time, agent_id, ram_total, ram_used, ram_available, ram_percent, swap_total, swap_used, swap_percent
FROM metrics_memory
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetDiskRange :many
SELECT time, agent_id, device, mountpoint, filesystem, disk_type, total_bytes, used_bytes, free_bytes, used_percent, inodes_total, inodes_used, inodes_percent
FROM metrics_disk
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetDiskIORange :many
SELECT time, agent_id, device, read_bytes, write_bytes, read_ops, write_ops, read_latency, write_latency, io_in_progress
FROM metrics_disk_io
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetNetworkRange :many
SELECT time, agent_id, interface, mac, mtu, speed, rx_bytes, rx_packets, rx_errors, rx_drops, tx_bytes, tx_packets, tx_errors, tx_drops
FROM metrics_network
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetTemperatureRange :many
SELECT time, agent_id, sensor, temperature, max_temp
FROM metrics_temperature
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetSystemRange :many
SELECT time, agent_id, uptime, process_count, user_count, boot_time
FROM metrics_system
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetContainerRange :many
SELECT time, agent_id, container_id, name, image, state, source, kind, cpu_percent, cpu_cores, memory_bytes, memory_limit, net_rx_bytes, net_tx_bytes
FROM metrics_container
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetWifiRange :many
SELECT time, agent_id, interface, ssid, bssid, frequency_mhz, signal_dbm, noise_dbm, bitrate_mbps, link_quality
FROM metrics_wifi
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;

-- name: GetPiRange :many
SELECT time, agent_id, metric_type, arm_freq_hz, core_freq_hz, gpu_freq_hz, core_volts, sdram_c_volts, sdram_i_volts, sdram_p_volts, soft_temp_limit, throttled, under_voltage, freq_capped, undervoltage_occurred, freq_cap_occurred, throttled_occurred, soft_temp_limit_occurred, gpu_mem_total, gpu_mem_used, gpu_temp
FROM metrics_pi
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
ORDER BY time DESC;
