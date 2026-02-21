-- name: UpsertCurrentCPU :exec
INSERT INTO current_metrics (agent_id, cpu_usage, load_normalized, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (agent_id) DO UPDATE
SET cpu_usage = EXCLUDED.cpu_usage,
    load_normalized = EXCLUDED.load_normalized,
    updated_at = NOW();

-- name: UpsertCurrentMemory :exec
INSERT INTO current_metrics (agent_id, ram_percent, swap_percent, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (agent_id) DO UPDATE
SET ram_percent = EXCLUDED.ram_percent,
    swap_percent = EXCLUDED.swap_percent,
    updated_at = NOW();

-- name: UpsertCurrentDiskMax :exec
INSERT INTO current_metrics (agent_id, disk_max_percent, updated_at)
VALUES ($1, (
    SELECT COALESCE(MAX(used_percent), 0)
    FROM metrics_disk
    WHERE agent_id = $1 AND time > NOW() - INTERVAL '5 minutes'
), NOW())
ON CONFLICT (agent_id) DO UPDATE
SET disk_max_percent = EXCLUDED.disk_max_percent,
    updated_at = NOW();

-- name: UpsertCurrentNetwork :exec
INSERT INTO current_metrics (agent_id, net_rx_bytes, net_tx_bytes, updated_at)
VALUES ($1, (
    SELECT COALESCE(SUM(rx_bytes), 0)
    FROM metrics_network
    WHERE agent_id = $1 AND time > NOW() - INTERVAL '15 seconds'
), (
    SELECT COALESCE(SUM(tx_bytes), 0)
    FROM metrics_network
    WHERE agent_id = $1 AND time > NOW() - INTERVAL '15 seconds'
), NOW())
ON CONFLICT (agent_id) DO UPDATE
SET net_rx_bytes = EXCLUDED.net_rx_bytes,
    net_tx_bytes = EXCLUDED.net_tx_bytes,
    updated_at = NOW();

-- name: UpsertCurrentTemperature :exec
INSERT INTO current_metrics (agent_id, max_temp, updated_at)
VALUES ($1, (
    SELECT COALESCE(MAX(temperature), 0)
    FROM metrics_temperature
    WHERE agent_id = $1 AND time > NOW() - INTERVAL '15 seconds'
), NOW())
ON CONFLICT (agent_id) DO UPDATE
SET max_temp = EXCLUDED.max_temp,
    updated_at = NOW();

-- name: UpsertCurrentSystem :exec
INSERT INTO current_metrics (agent_id, uptime, process_count, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (agent_id) DO UPDATE
SET uptime = EXCLUDED.uptime,
    process_count = EXCLUDED.process_count,
    updated_at = NOW();

-- name: UpsertCurrentReboot :exec
INSERT INTO current_metrics (agent_id, reboot_required, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (agent_id) DO UPDATE
SET reboot_required = EXCLUDED.reboot_required,
    updated_at = NOW();

-- name: GetOverview :many
SELECT  a.id, a.hostname, a.os, a.platform, a.arch, a.cpu_cores, a.last_seen,
        m.cpu_usage, m.load_normalized, m.ram_percent, m.swap_percent,
        m.disk_max_percent, m.net_rx_bytes, m.net_tx_bytes, m.max_temp,
        m.uptime, m.process_count, m.reboot_required, m.updated_at
FROM agents a
LEFT JOIN current_metrics m ON a.id = m.agent_id
ORDER BY a.hostname;