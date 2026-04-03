-- name: GetFleetHeatmap :many
WITH cpu_buckets AS (
    SELECT
        time_bucket('1 hour', c.time)::timestamptz AS bucket,
        agent_id,
        AVG(c.usage)::float8 AS value,
        'cpu'::text AS metric
    FROM metrics_cpu c
    WHERE c.time >= @start_time AND c.time <= @end_time
    GROUP BY 1, 2
),
mem_buckets AS (
    SELECT
        time_bucket('1 hour', m.time)::timestamptz AS bucket,
        agent_id,
        AVG(m.ram_percent)::float8 AS value,
        'mem'::text AS metric
    FROM metrics_memory m
    WHERE m.time >= @start_time AND m.time <= @end_time
    GROUP BY 1, 2
),
disk_buckets AS (
    SELECT
        time_bucket('1 hour', d.time)::timestamptz AS bucket,
        agent_id,
        MAX(d.used_percent)::float8 AS value,
        'disk'::text AS metric
    FROM metrics_disk d
    WHERE d.time >= @start_time AND d.time <= @end_time
    GROUP BY 1, 2
)
SELECT bucket, agent_id, value, metric
FROM cpu_buckets
UNION ALL
SELECT bucket, agent_id, value, metric
FROM mem_buckets
UNION ALL
SELECT bucket, agent_id, value, metric
FROM disk_buckets
ORDER BY agent_id, metric, bucket ASC;