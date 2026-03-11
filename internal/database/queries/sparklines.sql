-- name: GetRecentCPU :many
-- Returns the latest 30 CPU samples per agent, ordered chronologically.
SELECT agent_id, usage
FROM (
    SELECT
        agent_id,
        usage,
        time,
        ROW_NUMBER() OVER (
            PARTITION BY agent_id
            ORDER BY time DESC
        ) AS rn
    FROM metrics_cpu
) t
WHERE rn <= 30
ORDER BY agent_id, time ASC;

-- name: GetRecentMemory :many
-- Returns the latest 30 RAM samples per agent, ordered chronologically.
SELECT agent_id, ram_percent
FROM (
    SELECT
        agent_id,
        ram_percent,
        time,
        ROW_NUMBER() OVER (
            PARTITION BY agent_id
            ORDER BY time DESC
        ) AS rn
    FROM metrics_memory
) t
WHERE rn <= 30
ORDER BY agent_id, time ASC;

-- name: GetRecentDiskMax :many
-- Returns the latest 30 disk max samples per agent, ordered chronologically.
WITH disk_buckets AS (
    SELECT
        agent_id,
        date_trunc('minute', time) AS bucket,
        MAX(used_percent)::double precision AS max_percent
    FROM metrics_disk
    GROUP BY agent_id, date_trunc('minute', time)
),
ranked AS (
    SELECT
        agent_id,
        max_percent,
        bucket,
        ROW_NUMBER() OVER (
            PARTITION BY agent_id
            ORDER BY bucket DESC
        ) AS rn
    FROM disk_buckets
)
SELECT agent_id, max_percent
FROM ranked
WHERE rn <= 30
ORDER BY agent_id, bucket ASC;