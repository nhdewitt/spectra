-- name: GetLatestCPU :many
SELECT time, agent_id, usage, core_usages, load_1m, load_5m, load_15m, iowait
FROM metrics_cpu
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestMemory :many
SELECT time, agent_id, ram_total, ram_used, ram_available, ram_percent, swap_total, swap_used, swap_percent
FROM metrics_memory
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestDisk :many
SELECT time, agent_id, device, mountpoint, filesystem, disk_type, total_bytes, used_bytes, free_bytes, used_percent, inodes_total, inodes_used, inodes_percent
FROM metrics_disk
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestDiskIO :many
SELECT time, agent_id, device, read_bytes, write_bytes, read_ops, write_ops, read_latency, write_latency, io_in_progress
FROM metrics_disk_io
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestNetwork :many
SELECT time, agent_id, interface, mac, mtu, speed, rx_bytes, rx_packets, rx_errors, rx_drops, tx_bytes, tx_packets, tx_errors, tx_drops
FROM metrics_network
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestTemperature :many
SELECT time, agent_id, sensor, temperature, max_temp
FROM metrics_temperature
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;

-- name: GetLatestSystem :one
SELECT time, agent_id, uptime, process_count, user_count, boot_time
FROM metrics_system
WHERE agent_id = $1
ORDER BY time DESC
LIMIT 1;

-- name: GetLatestContainers :many
SELECT time, agent_id, container_id, name, image, state, source, kind, cpu_percent, cpu_cores, memory_bytes, memory_limit, net_rx_bytes, net_tx_bytes
FROM metrics_container
WHERE agent_id = $1 AND time > NOW() - $2::interval
ORDER BY time DESC;