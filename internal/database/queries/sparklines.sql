-- name: GetRecentCPU :many
-- Returns last 5 minutes of CPU usage for all agents, ordered chronologically.
SELECT agent_id, time, usage
FROM metrics_cpu
WHERE time >= NOW() - INTERVAL '5 minutes'
ORDER BY agent_id, time ASC;

-- name: GetRecentMemory :many
-- Returns last 5 minutes of RAM percent for all agents, ordered chronologically.
SELECT agent_id, time, ram_percent
FROM metrics_memory
WHERE time >= NOW() - INTERVAL '5 minutes'
ORDER BY agent_id, time ASC;

-- name: GetRecentDiskMax :many
-- Returns last 5 minutes of max disk usage per timestamp per agent.
SELECT agent_id, time, CAST(MAX(used_percent) AS DOUBLE PRECISION) AS max_percent
FROM metrics_disk
WHERE time >= NOW() - INTERVAL '5 minutes'
GROUP BY agent_id, time
ORDER BY agent_id, time ASC;