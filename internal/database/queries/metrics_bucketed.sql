-- Bucketed metric queries using TimescaleDB time_bucket.
-- The interval parameter controls aggregation granularity.
-- Handlers pick the bucket size based on the time range:
--  <= 1h: no bucketing
--  <= 6h: 1m
--  <= 24h: 5m
--  <= 7d: 15m
--  <= 30d: 1h

-- name: GetCPUBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    AVG(usage)::float8 AS usage,
    NULL::float8[] AS core_usages,
    AVG(load_1m)::float8 AS load_1m,
    AVG(load_5m)::float8 AS load_5m,
    AVG(load_15m)::float8 AS load_15m,
    AVG(iowait)::float8 AS iowait
FROM metrics_cpu
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2
ORDER BY 1 ASC;

-- name: GetMemoryBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    AVG(ram_total)::float8 AS ram_total,
    AVG(ram_used)::float8 AS ram_used,
    AVG(ram_available)::float8 AS ram_available,
    AVG(ram_percent)::float8 AS ram_percent,
    AVG(swap_total)::float8 AS swap_total,
    AVG(swap_used)::float8 AS swap_used,
    AVG(swap_percent)::float8 AS swap_percent
FROM metrics_memory
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2
ORDER BY 1 ASC;

-- name: GetDiskBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    device,
    mountpoint,
    filesystem,
    disk_type,
    AVG(total_bytes)::float8 AS total_bytes,
    AVG(used_bytes)::float8 AS used_bytes,
    AVG(free_bytes)::float8 AS free_bytes,
    AVG(used_percent)::float8 AS used_percent,
    AVG(inodes_total)::float8 AS inodes_total,
    AVG(inodes_used)::float8 AS inodes_used,
    AVG(inodes_percent)::float8 AS inodes_percent
FROM metrics_disk
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3, 4, 5, 6
ORDER BY 1 ASC;

-- name: GetDiskIOBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    device,
    AVG(read_bytes)::float8 AS read_bytes,
    AVG(write_bytes)::float8 AS write_bytes,
    AVG(read_ops)::float8 AS read_ops,
    AVG(write_ops)::float8 AS write_ops,
    AVG(read_latency)::float8 AS read_latency,
    AVG(write_latency)::float8 AS write_latency,
    AVG(io_in_progress)::float8 AS io_in_progress
FROM metrics_disk_io
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3
ORDER BY 1 ASC;

-- name: GetNetworkBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    interface,
    MAX(mac)::text AS mac,
    MAX(mtu)::bigint AS mtu,
    MAX(speed)::bigint AS speed,
    AVG(rx_bytes)::float8 AS rx_bytes,
    AVG(rx_packets)::float8 AS rx_packets,
    AVG(rx_errors)::float8 AS rx_errors,
    AVG(rx_drops)::float8 AS rx_drops,
    AVG(tx_bytes)::float8 AS tx_bytes,
    AVG(tx_packets)::float8 AS tx_packets,
    AVG(tx_errors)::float8 AS tx_errors,
    AVG(tx_drops)::float8 AS tx_drops
FROM metrics_network
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3
ORDER BY 1 ASC;

-- name: GetTemperatureBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    sensor,
    AVG(temperature)::float8 AS temperature,
    COALESCE(MAX(max_temp), 0)::float8 AS max_temp
FROM metrics_temperature
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3
ORDER BY 1 ASC;

-- name: GetSystemBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    AVG(uptime)::float8 AS uptime,
    AVG(process_count)::float8 AS process_count,
    AVG(user_count)::float8 AS user_count,
    MAX(boot_time)::timestamptz AS boot_time
FROM metrics_system
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2
ORDER BY 1 ASC;

-- name: GetContainerBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    container_id,
    MAX(name)::text AS name,
    MAX(image)::text AS image,
    MAX(state)::text AS state,
    MAX(source)::text AS source,
    MAX(kind)::text AS kind,
    AVG(cpu_percent)::float8 AS cpu_percent,
    AVG(cpu_cores)::float8 AS cpu_cores,
    AVG(memory_bytes)::float8 AS memory_bytes,
    AVG(memory_limit)::float8 AS memory_limit,
    AVG(net_rx_bytes)::float8 AS net_rx_bytes,
    AVG(net_tx_bytes)::float8 AS net_tx_bytes
FROM metrics_container
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3
ORDER BY 1 ASC;
 
-- name: GetWifiBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    interface,
    MAX(ssid)::text AS ssid,
    MAX(bssid)::text AS bssid,
    AVG(frequency_mhz)::float8 AS frequency_mhz,
    AVG(signal_dbm)::float8 AS signal_dbm,
    AVG(noise_dbm)::float8 AS noise_dbm,
    AVG(bitrate_mbps)::float8 AS bitrate_mbps,
    AVG(link_quality)::float8 AS link_quality
FROM metrics_wifi
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2, 3
ORDER BY 1 ASC;
 
-- name: GetPiBucketed :many
SELECT
    time_bucket((@bucket_interval)::text::interval, time)::timestamptz AS time,
    agent_id,
    MAX(metric_type)::text AS metric_type,
    AVG(arm_freq_hz)::float8 AS arm_freq_hz,
    AVG(core_freq_hz)::float8 AS core_freq_hz,
    AVG(gpu_freq_hz)::float8 AS gpu_freq_hz,
    AVG(core_volts)::float8 AS core_volts,
    AVG(sdram_c_volts)::float8 AS sdram_c_volts,
    AVG(sdram_i_volts)::float8 AS sdram_i_volts,
    AVG(sdram_p_volts)::float8 AS sdram_p_volts,
    AVG(soft_temp_limit)::float8 AS soft_temp_limit,
    BOOL_OR(throttled) AS throttled,
    BOOL_OR(under_voltage) AS under_voltage,
    BOOL_OR(freq_capped) AS freq_capped,
    BOOL_OR(undervoltage_occurred) AS undervoltage_occurred,
    BOOL_OR(freq_cap_occurred) AS freq_cap_occurred,
    BOOL_OR(throttled_occurred) AS throttled_occurred,
    BOOL_OR(soft_temp_limit_occurred) AS soft_temp_limit_occurred,
    AVG(gpu_mem_total)::float8 AS gpu_mem_total,
    AVG(gpu_mem_used)::float8 AS gpu_mem_used,
    AVG(gpu_temp)::float8 AS gpu_temp
FROM metrics_pi
WHERE agent_id = @agent_id AND time >= @start_time AND time <= @end_time
GROUP BY 1, 2
ORDER BY 1 ASC;