ALTER TABLE metrics_disk_io
    DROP COLUMN IF EXISTS read_latency_ms,
    DROP COLUMN IF EXISTS write_latency_ms,
    DROP COLUMN IF EXISTS read_busy_pct,
    DROP COLUMN IF EXISTS write_busy_pct;