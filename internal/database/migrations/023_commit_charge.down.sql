ALTER TABLE metrics_memory
    DROP COLUMN IF EXISTS commit_limit,
    DROP COLUMN IF EXISTS commit_used;