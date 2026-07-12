-- name: GetStatusThresholds :one
-- Load the global state thresholds. The table holds a single row (id = 1,
-- enforced by a CHECK), seeded with defaults by the migration, so this read
-- always returns exactly one row.
SELECT cpu_warn, cpu_crit, mem_warn, mem_crit, disk_warn, disk_crit,
	   temp_warn, temp_crit, stale_seconds, offline_seconds, updated_at
FROM status_thresholds
WHERE id = 1;

-- name: UpsertStatusThresholds :exec
-- Replace the global status thresholds. Targets the singleton row (id = 1);
-- the ON CONFLICT branch updates in place when the row already exists, which it
-- always does after the migration seed.
INSERT INTO status_thresholds (
	id, cpu_warn, cpu_crit, mem_warn, mem_crit, disk_warn, disk_crit,
	temp_warn, temp_crit, stale_seconds, offline_seconds, updated_at
) VALUES (
	1, @cpu_warn, @cpu_crit, @mem_warn, @mem_crit, @disk_warn, @disk_crit,
	@temp_warn, @temp_crit, @stale_seconds, @offline_seconds, NOW()
)
ON CONFLICT (id) DO UPDATE
SET cpu_warn = EXCLUDED.cpu_warn,
    cpu_crit = EXCLUDED.cpu_crit,
    mem_warn = EXCLUDED.mem_warn,
    mem_crit = EXCLUDED.mem_crit,
    disk_warn = EXCLUDED.disk_warn,
    disk_crit = EXCLUDED.disk_crit,
    temp_warn = EXCLUDED.temp_warn,
    temp_crit = EXCLUDED.temp_crit,
    stale_seconds = EXCLUDED.stale_seconds,
    offline_seconds = EXCLUDED.offline_seconds,
    updated_at = NOW();	